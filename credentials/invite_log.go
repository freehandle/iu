package credentials

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Formato de cada linha do arquivo:
//
//	CRIADO|{seed}|{inviter}|{appname}|{criado_em RFC3339}
//	ACEITO|{seed}|{invitee}|{aceito_em RFC3339}

// RegistroConvite armazena os dados de um convite, pendente ou aceito.
type RegistroConvite struct {
	Seed          string
	InviterHandle string
	InviteeHandle string    // vazio se ainda pendente
	AppName       string
	CriadoEm     time.Time
	AceitoEm     time.Time // zero se pendente
}

// Pendente retorna true se o convite ainda não foi aceito.
func (r *RegistroConvite) Pendente() bool {
	return r.AceitoEm.IsZero()
}

// InviteLog é o registro persistido em arquivo de texto de todos os convites.
type InviteLog struct {
	mu        sync.Mutex
	file      *os.File
	registros map[string]*RegistroConvite // seed → registro
}

// AbrirInviteLog abre (ou cria) o arquivo de texto e carrega os registros existentes.
func AbrirInviteLog(caminho string) (*InviteLog, error) {
	file, err := os.OpenFile(caminho, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, fmt.Errorf("invite log: não foi possível abrir %s: %w", caminho, err)
	}

	il := &InviteLog{
		file:      file,
		registros: make(map[string]*RegistroConvite),
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		linha := scanner.Text()
		if linha == "" {
			continue
		}
		campos := strings.Split(linha, "|")
		switch campos[0] {
		case "CRIADO":
			if len(campos) == 5 {
				t, _ := time.Parse(time.RFC3339, campos[4])
				il.registros[campos[1]] = &RegistroConvite{
					Seed:          campos[1],
					InviterHandle: campos[2],
					AppName:       campos[3],
					CriadoEm:     t,
				}
			}
		case "ACEITO":
			if len(campos) == 4 {
				t, _ := time.Parse(time.RFC3339, campos[3])
				if r, ok := il.registros[campos[1]]; ok {
					r.InviteeHandle = campos[2]
					r.AceitoEm = t
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		file.Close()
		return nil, fmt.Errorf("invite log: erro ao ler arquivo: %w", err)
	}

	return il, nil
}

// RegistrarConvite grava no arquivo que um convite foi criado.
func (il *InviteLog) RegistrarConvite(seed, inviterHandle, appName string) {
	il.mu.Lock()
	defer il.mu.Unlock()

	criadoEm := time.Now()
	il.registros[seed] = &RegistroConvite{
		Seed:          seed,
		InviterHandle: inviterHandle,
		AppName:       appName,
		CriadoEm:     criadoEm,
	}

	linha := fmt.Sprintf("CRIADO|%s|%s|%s|%s\n",
		seed, inviterHandle, appName, criadoEm.Format(time.RFC3339))
	il.escrever(linha)
}

// RegistrarAceite grava no arquivo que um convite foi aceito.
func (il *InviteLog) RegistrarAceite(seed, inviteeHandle string) {
	il.mu.Lock()
	defer il.mu.Unlock()

	aceitoEm := time.Now()
	if r, ok := il.registros[seed]; ok {
		r.InviteeHandle = inviteeHandle
		r.AceitoEm = aceitoEm
	}

	linha := fmt.Sprintf("ACEITO|%s|%s|%s\n",
		seed, inviteeHandle, aceitoEm.Format(time.RFC3339))
	il.escrever(linha)
}

func (il *InviteLog) escrever(linha string) {
	il.file.Seek(0, 2) // append
	if _, err := il.file.WriteString(linha); err != nil {
		fmt.Printf("invite log: erro ao gravar: %v\n", err)
	}
}

// Registros retorna todos os registros de convite.
func (il *InviteLog) Registros() []*RegistroConvite {
	il.mu.Lock()
	defer il.mu.Unlock()
	lista := make([]*RegistroConvite, 0, len(il.registros))
	for _, r := range il.registros {
		lista = append(lista, r)
	}
	return lista
}

// Close fecha o arquivo de log.
func (il *InviteLog) Close() {
	il.file.Close()
}
