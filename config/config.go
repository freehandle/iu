package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/freehandle/breeze/crypto"
	bconfig "github.com/freehandle/breeze/middleware/config"
	"github.com/freehandle/breeze/middleware/simple"
	"github.com/freehandle/iu/auth"
)

type ConfigLocal struct {
	BlocksPath       string                  `json:"blocks_path"`        // caminho para os blocos
	SecretPath       string                  `json:"secret_path"`        // caminho para o arquivo .pem com a chave privada
	GatewayToken     string                  `json:"gateway_token"`      // token do gateway do breeze
	GatewayPort      int                     `json:"gateway_port"`       // numero da porta do gateway para conexao com o breeze
	AppDataPath      string                  `json:"app_data_path"`      // pasta onde ficam o cookies.dat e o passwords.dat da aplicacao
	HostName         string                  `json:"host_name"`          // nome do site
	Mucua            string                  `json:"mucua"`              // caminho que esta servindo a aplicaco
	EmailTemplates   *auth.MessagesTemplates `json:"email_templates"`    // templates de email de envio
	EmailAddress     string                  `json:"email_address"`      // endereco de email que faz os envios
	EnvEmailPassword string                  `json:"env_email_password"` // nome da variavel de ambiente onde estara a senha para envio dos emails
	AppName          string                  `json:"app_name"`           // nome da aplicacao
	SafeAddress      string                  `json:"safe_address"`
	SafeAPIAddress   string                  `json:"safe_api_address"`
	GenesisTime      string                  `json:"genesis_time"`
	UserFilesPath    string                  `json:"user_files_path"`
	AppPort          int                     `json:"app_port"` // numero da porta onde a aplicacao sera servida
}

func (c ConfigLocal) Check() error {
	return nil
}

func DefaultTemplate(appName string) *auth.MessagesTemplates {
	mensagens := auth.MessagesTemplates{
		Reset:                    "To reset your password, click the following link: %s",
		ResetHeader:              "Password Reset",
		Signin:                   "To sign in, click the following link: %s",
		SigninHeader:             "Sign In",
		Wellcome:                 "Welcome! Your account has been created.",
		WellcomeHeader:           "Welcome to our App",
		EmailSigninMessage:       "To sign in without your handle, click the following link: %s",
		EmailSigninMessageHeader: "Sign In Without Handle",
		PasswordMessage:          "Your new password is: %s",
		PasswordMessageHeader:    "New Password",
		VerifyPOAHeader:          fmt.Sprintf("%s Confirmação de email", appName),
		VerifyPOA:                "Foi requerida a autorização de uso do seu handle %v para a aplicação %v. Se não foi você quem requisitou, por favor, ignore esta mensagem.\n\nPara autorizar, por favor, clique no link abaixo:\n\n%s",
	}
	return &mensagens

}

type Resources struct {
	Secret      crypto.PrivateKey // segredo
	Gateway     chan []byte
	Blocks      chan []byte
	Manager     *auth.SigninManager
	Context     context.Context
	GenesisTime time.Time
	Config      ConfigLocal
}

func RunConfig(cfg ConfigLocal) (*Resources, error) {
	// Token e Private Key
	token, pk := crypto.RandomAsymetricKey()
	if cfg.SecretPath != "" {
		data, err := os.ReadFile(cfg.SecretPath)
		if err != nil {
			return nil, fmt.Errorf("could not read credentials file: %v", err)
		}
		pk, err = crypto.ParsePEMPrivateKey(data)
		if err != nil {
			return nil, fmt.Errorf("could not parse credentials file: %v", err)
		}
		token = pk.PublicKey()
	}
	// Gateway
	ctx := context.Background()
	gatewayToken := crypto.TokenFromString(cfg.GatewayToken)
	sender, err := simple.Gateway(ctx, cfg.GatewayPort, gatewayToken, pk)
	if err != nil {
		return nil, fmt.Errorf("could not connect to gateway: %v", err)
	}
	// Leitor blocos
	blocks := simple.DissociateActions(ctx, simple.NewBlockReader(ctx, cfg.BlocksPath, "blocos", time.Second))

	// Tempo Genesis
	timeGenesis, err := time.Parse("2006-01-02T15:04:05.0000", cfg.GenesisTime) //  "Mon Jan 2 15:04:05 MST 2006"
	if err != nil {
		return nil, fmt.Errorf("could not read genesis time: %v", err)
	}

	arqCofrinho := fmt.Sprintf("%s/%s", cfg.AppDataPath, "senhas.dat")
	cofrinho := auth.NewFilePasswordManager(arqCofrinho)

	envs := os.Environ()
	var senhaEmail string
	for _, env := range envs {
		if strings.HasPrefix(env, cfg.EnvEmailPassword) {
			senhaEmail, _ = strings.CutPrefix(env, fmt.Sprintf("%s=", cfg.EnvEmailPassword))
		}
	}

	var gmail auth.Mailer
	if cfg.EmailAddress == "" {
		gmail = auth.TesteGmail{}
	} else {
		gmail = &auth.SMTPGmail{
			Password: senhaEmail,
			From:     cfg.EmailAddress,
		}
	}

	if cfg.EmailTemplates == nil {
		cfg.EmailTemplates = DefaultTemplate(cfg.AppName)
	}
	carteiro := &auth.SMTPManager{
		Mail:      gmail,
		Token:     token,
		Templates: *cfg.EmailTemplates,
	}

	arqDoceria := fmt.Sprintf("%s/%s", cfg.AppDataPath, "cookies.dat")
	doceria, err := auth.OpenCokieStore(arqDoceria)
	if err != nil {
		return nil, fmt.Errorf("could not create cookie store: %v", err)
	}

	gerente := &auth.SigninManager{
		AppName:        cfg.AppName,
		Passwords:      cofrinho,
		Cookies:        doceria,
		Mail:           carteiro,
		Granted:        make(map[string]crypto.Token),
		Credentials:    pk,
		Members:        &auth.DefaultAssociater{AplicationName: cfg.AppName, AppToken: token},
		SafeAddress:    cfg.SafeAddress,
		SafeAPIAddress: cfg.SafeAPIAddress,
		HandleToToken:  make(map[string]crypto.Token),
		TokenToHandle:  make(map[crypto.Token]string),
	}

	return &Resources{
		Secret:      pk,
		Gateway:     sender,
		Blocks:      blocks,
		Manager:     gerente,
		Context:     ctx,
		GenesisTime: timeGenesis,
		Config:      cfg,
	}, nil

}

func LoadConfig(path string) (*Resources, error) {
	cfg, err := bconfig.LoadConfig[ConfigLocal](path)
	if err != nil {
		return nil, err
	}
	return RunConfig(*cfg)
}
