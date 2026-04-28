package conn

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/middleware/simple"
	"github.com/freehandle/breeze/socket"
	"github.com/freehandle/breeze/util"
	"github.com/freehandle/handles"
	"github.com/freehandle/handles/attorney"
)

// Node é a abstração de conectividade do iu com a rede handles/breeze.
// Fornece envio de ações, rastreamento de epoch, fonte para auth.LaunchManager
// e entrega de voids filtrados pelo protocolo social do app.
type Node interface {
	// Send envia uma ação assinada para a chain.
	Send(action []byte)

	// Epoch retorna o epoch atual da chain.
	Epoch() uint64

	// AuthSource retorna o canal de mensagens para auth.LaunchManager:
	// epoch updates ([0,0,epoch...]) e ações handles (Join, Grant, Revoke).
	AuthSource() <-chan []byte

	// Voids retorna o canal de voids cujo void.Data passou o filtro do protocolo.
	Voids() <-chan *attorney.Void

	// Shutdown encerra a conexão com a chain.
	Shutdown()
}

// baseNode contém o estado compartilhado entre SimpleNode e BreezeNode.
type baseNode struct {
	epoch      uint64
	mu         sync.RWMutex
	authSource chan []byte
	voids      chan *attorney.Void
	shutdown   func()
}

func (b *baseNode) Epoch() uint64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.epoch
}

func (b *baseNode) AuthSource() <-chan []byte      { return b.authSource }
func (b *baseNode) Voids() <-chan *attorney.Void   { return b.voids }
func (b *baseNode) Shutdown()                      { b.shutdown() }

func (b *baseNode) setEpoch(epoch uint64) {
	b.mu.Lock()
	b.epoch = epoch
	b.mu.Unlock()
}

// processBlock faz o fan-out de um HandlesBlock para authSource e voids.
// filter é chamada com void.Data; retorna true para encaminhar o void.
func (b *baseNode) processBlock(block *handles.HandlesBlock, filter func([]byte) bool) {
	epochMsg := []byte{0, 0}
	util.PutUint64(block.Epoch, &epochMsg)
	b.setEpoch(block.Epoch)
	b.authSource <- epochMsg

	for _, join := range block.Join {
		b.authSource <- append([]byte{1}, join.Serialize()...)
	}
	for _, grant := range block.Grant {
		b.authSource <- append([]byte{1}, grant.Serialize()...)
	}
	for _, revoke := range block.Revoke {
		b.authSource <- append([]byte{1}, revoke.Serialize()...)
	}
	for _, void := range block.Void {
		if filter(void.Data) {
			b.voids <- void
		}
	}
}

// SimpleNode conecta a um nó SimpleChain lendo blocos de arquivo local.
type SimpleNode struct {
	baseNode
	send chan []byte
}

func (n *SimpleNode) Send(action []byte) { n.send <- action }

// NewSimpleNode cria um Node para um nó SimpleChain.
//   - port/token/credentials: gateway TCP para envio de ações
//   - dataPath/blockName: localização dos arquivos de bloco no disco
//   - filter: chamada com void.Data; retorna true para encaminhar o void ao protocolo
func NewSimpleNode(ctx context.Context, port int, token crypto.Token, credentials crypto.PrivateKey, dataPath, blockName string, filter func([]byte) bool) (*SimpleNode, error) {
	send, err := simple.Gateway(ctx, port, token, credentials)
	if err != nil {
		return nil, fmt.Errorf("conn: SimpleNode gateway dial: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	n := &SimpleNode{
		baseNode: baseNode{
			authSource: make(chan []byte, 10),
			voids:      make(chan *attorney.Void, 10),
			shutdown:   cancel,
		},
		send: send,
	}

	blocks := handles.LocalHandleListener(ctx, dataPath, blockName, time.Second)
	go func() {
		defer func() {
			close(n.authSource)
			close(n.voids)
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case block, ok := <-blocks:
				if !ok {
					return
				}
				n.processBlock(block, filter)
			}
		}
	}()

	return n, nil
}

// BreezeNode conecta a um nó da rede Breeze via TrustedAggregator.
type BreezeNode struct {
	baseNode
	conn *socket.SignedConnection
}

func (n *BreezeNode) Send(action []byte) {
	if err := n.conn.Send(action); err != nil {
		log.Printf("conn: BreezeNode send error: %v", err)
	}
}

// NewBreezeNode cria um Node para a rede Breeze distribuída.
//   - gateway: endereço e token do nó gateway para envio de ações
//   - credentials: chave privada do app
//   - providers: provedores de blocos para TrustedAggregator
//   - filter: chamada com void.Data; retorna true para encaminhar o void ao protocolo
func NewBreezeNode(ctx context.Context, gateway socket.TokenAddr, credentials crypto.PrivateKey, providers []socket.TokenAddr, filter func([]byte) bool) (*BreezeNode, error) {
	conn, err := socket.Dial("", gateway.Addr, credentials, gateway.Token)
	if err != nil {
		return nil, fmt.Errorf("conn: BreezeNode gateway dial: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	n := &BreezeNode{
		baseNode: baseNode{
			authSource: make(chan []byte, 10),
			voids:      make(chan *attorney.Void, 10),
			shutdown:   cancel,
		},
		conn: conn,
	}

	sources := socket.NewTrustedAgregator(ctx, "", credentials, 1, providers, nil)
	blocks := handles.HandlesListener(ctx, sources)
	go func() {
		defer func() {
			conn.Shutdown()
			close(n.authSource)
			close(n.voids)
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case block, ok := <-blocks:
				if !ok {
					return
				}
				n.processBlock(block, filter)
			}
		}
	}()

	return n, nil
}
