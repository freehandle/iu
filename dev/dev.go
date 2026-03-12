// Package dev fornece um stack local de desenvolvimento com proxy-handles e safe
// embutidos no mesmo processo, conectados via TCP loopback.
//
// Uso típico em um protocolo social:
//
//	stack, err := dev.Start(ctx, "./dev-data")
//	if err != nil { log.Fatal(err) }
//	gerente, err := stack.NovoGerente(ctx, aplicacao, credenciais)
package dev

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/middleware/simple"
	"github.com/freehandle/breeze/socket"
	"github.com/freehandle/breeze/util"
	"github.com/freehandle/handles/attorney"
	"github.com/freehandle/iu/auth"
	"github.com/freehandle/safe"
)

// chave hardcoded do proxy-handles para desenvolvimento
var chainPK = crypto.PrivateKeyFromString("b61b1452f41a62ac20a1cf5136a2c692d7312f70c65f196426f0a11aca733d3d91ad274d06c4be307a332a0e59449ad25ae2c65e4ad5a8f0af87067ac2fc3a54")

// appPK é uma chave estável para o app em dev — garante que GrantPowerOfAttorney
// gravado nos blocos continue válido entre restarts.
var appPK = crypto.PrivateKeyFromString("a72b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2")

// AppDevKey retorna as credenciais estáveis de desenvolvimento do app.
// Use no main.go do protocolo em vez de crypto.RandomAsymetricKey().
func AppDevKey() crypto.PrivateKey {
	return appPK
}

// cookieName converte o nome do app em nome de cookie válido (sem espaços).
func cookieName(appName string) string {
	name := make([]byte, 0, len(appName)+8)
	for _, c := range []byte(appName) {
		if c == ' ' {
			name = append(name, '_')
		} else {
			name = append(name, c)
		}
	}
	return string(append(name, []byte("_session")...))
}

// noopGateway satisfaz auth.Gateway sem fazer nada (gerente não envia ações diretamente em dev).
type noopGateway struct{}

func (noopGateway) Send(action []byte) {}
func (noopGateway) Epoch() uint64      { return 0 }

const (
	ChainPort  = 7000
	SafePort   = 4100
	SafeAPI    = 4101
	SafePasswd = "devpassword"
)

// LocalStack representa o stack de desenvolvimento em execução.
type LocalStack struct {
	// SafeAPIAddress é o endereço HTTP da REST API do safe.
	SafeAPIAddress string

	// ChainToken é o token público do proxy-handles.
	ChainToken crypto.Token

	dataPath     string
	done         chan error
	safeInstance *safe.Safe
}

// Wait bloqueia até que algum componente do stack encerre.
func (s *LocalStack) Wait() error {
	return <-s.done
}

// blocksToActions converte SimpleBlocks num canal no formato que LaunchManager espera:
// [0][0][epoch_8bytes] para atualizações de época, [1][action_bytes] para ações.
func blocksToActions(ctx context.Context, dataPath string) chan []byte {
	blocks := simple.NewBlockReader(ctx, dataPath, "blocos", time.Second)
	out := make(chan []byte, 10)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case block, ok := <-blocks:
				if !ok {
					return
				}
				epochMsg := []byte{0, 0}
				util.PutUint64(block.Epoch, &epochMsg)
				out <- epochMsg
				for _, action := range block.Actions {
					out <- append([]byte{1}, action...)
				}
			}
		}
	}()
	return out
}

// NovoGerente cria um SigninManager usando LaunchManager, que reconstrói
// HandleToToken/TokenToHandle/Granted a partir dos blocos gravados em disco
// e continua atualizando conforme novos blocos chegam.
// - Emails são impressos no stdout (TesteGmail).
// - SafeAPIAddress já aponta para o safe local.
func (s *LocalStack) NovoGerente(ctx context.Context, members auth.Associater, credentials crypto.PrivateKey) (*auth.SigninManager, error) {
	senhas := auth.NewFilePasswordManager(s.dataPath + "/senhas.dat")

	cookies, err := auth.OpenCokieStore(s.dataPath + "/cookies.dat")
	if err != nil {
		return nil, fmt.Errorf("dev: could not open cookie store: %v", err)
	}

	cfg := auth.ManagerConfig{
		Token:     members.AttorneyToken(),
		Passwords: senhas,
		Mail:      auth.TesteGmail{},
		Gateway:   noopGateway{},
		Templates: auth.MessagesTemplates{},
	}

	source := blocksToActions(ctx, s.dataPath)
	gerente, _ := auth.LaunchManager(ctx, cfg, source)

	gerente.AppName        = members.AppName()
	gerente.CookieName     = cookieName(members.AppName())
	gerente.Secure         = false
	gerente.AppToken       = members.AttorneyToken()
	gerente.Cookies        = cookies
	gerente.Credentials    = credentials
	gerente.Members        = members
	gerente.SafeAPIAddress = s.SafeAPIAddress

	// Pre-popula HandleToToken e TokenToHandle diretamente do vault do safe,
	// que já carregou todos os usuários registrados do disco.
	if s.safeInstance != nil {
		for handle, token := range s.safeInstance.HandleTokens() {
			gerente.HandleToToken[handle] = token
			gerente.TokenToHandle[token] = handle
		}
		log.Printf("[dev] gerente pré-populado com %d usuários do vault", len(gerente.HandleToToken))
	}

	return gerente, nil
}

// Start inicia o proxy-handles (SimpleChain) e o safe em modo dev no mesmo processo.
// dataPath: diretório para persistência (vault, blocos, cookies).
func Start(ctx context.Context, dataPath string) (*LocalStack, error) {
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return nil, fmt.Errorf("dev: could not create data dir: %v", err)
	}

	// 1. Lê blocos existentes e reconstrói estado genesis
	genesis := attorney.NewGenesisState("")
	blocos := make(chan *simple.SimpleBlock, 1)
	lastEpoch := make(chan uint64, 1)
	go func() {
		epoch := uint64(0)
		for block := range blocos {
			validator := genesis.Validator()
			epoch = block.Epoch
			for _, action := range block.Actions {
				validator.Validate(action)
			}
			genesis.Incorporate(validator.Mutations())
		}
		lastEpoch <- epoch
	}()

	writer, err := simple.OpenSimpleBlockWriter(dataPath, "blocos", 100000000, blocos)
	if err != nil {
		return nil, fmt.Errorf("dev: could not open block writer: %v", err)
	}
	epoch := <-lastEpoch
	log.Printf("[dev] proxy-handles: blocos lidos até epoch %d", epoch)

	// 2. Inicia proxy-handles (SimpleChain) na porta ChainPort
	chainToken := chainPK.PublicKey()
	chain := simple.SimpleChain[*attorney.Mutations, *attorney.MutatingState]{
		Interval:    time.Second,
		GatewayPort: ChainPort,
		Writer:      writer,
		State:       genesis,
		Epoch:       epoch,
		Recent:      [][][]byte{},
		Keep:        10,
	}
	chainErr := chain.Start(ctx, chainPK, attorney.ToString)
	log.Printf("[dev] proxy-handles na porta %d", ChainPort)

	time.Sleep(100 * time.Millisecond)

	// 3. Inicia safe em modo dev (apenas REST API, sem UI web)
	_, safePK := crypto.RandomAsymetricKey()
	safeCfg := safe.SafeConfig{
		Credentials: safePK,
		Path:        dataPath,
		Port:        SafePort,
		RestAPIPort: SafeAPI,
		Address:     fmt.Sprintf("localhost:%d", SafePort),
	}
	gatewayCfg := safe.GatewayConfig{
		Gateway:     socket.TokenAddr{Token: chainToken, Addr: fmt.Sprintf(":%d", ChainPort)},
		Credentials: safePK,
		Simple: &safe.SimpleBlockProvider{
			Path:     dataPath,
			Name:     "blocos",
			Interval: time.Second,
		},
	}
	safeErr, safeInstance := safe.NewDevServer(ctx, safeCfg, gatewayCfg, SafePasswd)

	done := make(chan error, 1)
	go func() {
		select {
		case err := <-chainErr:
			done <- fmt.Errorf("dev proxy-handles: %w", err)
		case err := <-safeErr:
			done <- fmt.Errorf("dev safe: %w", err)
		}
	}()

	return &LocalStack{
		SafeAPIAddress: fmt.Sprintf("http://localhost:%d", SafeAPI),
		ChainToken:     chainToken,
		dataPath:       dataPath,
		done:           done,
		safeInstance:   safeInstance,
	}, nil
}
