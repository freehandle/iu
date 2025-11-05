package auth

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"os"
	"sync"
	"time"

	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/util"
)

const (
	PasswordEntry = iota
	ResetPasswordEntry
)

type passwordReset struct {
	user   crypto.Token
	expire time.Time
	used   bool
}

type hashmail struct {
	hash  crypto.Hash
	email string
}

type filePasswordManager struct {
	mu        sync.Mutex
	file      os.File
	hashes    []hashmail
	passwords map[crypto.Token]int
	reset     map[crypto.Hash]passwordReset
}

func (f *filePasswordManager) HasReset(url string) (bool, crypto.Token, string) {
	decoded, err := base64.RawURLEncoding.DecodeString(url)
	if err != nil {
		return false, crypto.Token{}, ""
	}
	hash := crypto.Hasher(decoded)
	if reset, ok := f.reset[hash]; ok {
		if n, ok := f.passwords[reset.user]; ok {
			//delete(f.reset, hash)
			if (!reset.used) && n < len(f.hashes) && reset.expire.After(time.Now()) {
				f.reset[hash] = passwordReset{user: reset.user, expire: reset.expire, used: true}
				return true, reset.user, f.hashes[n].email
			}
		}
	}
	return false, crypto.Token{}, ""
}

func (f *filePasswordManager) AddReset(user crypto.Token, email string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	secret := make([]byte, 64)
	_, err := rand.Read(secret)
	if err != nil {
		return ""
	}
	url := base64.RawURLEncoding.EncodeToString(secret)
	hash := crypto.Hasher(secret)
	reset := passwordReset{
		user:   user,
		expire: time.Now().Add(3 * time.Hour),
	}
	data := []byte{ResetPasswordEntry}
	util.PutTime(reset.expire, &data)
	util.PutToken(user, &data)
	dressed := make([]byte, 0)
	util.PutByteArray(data, &dressed)
	f.file.Seek(0, 2)
	if n, err := f.file.Write(dressed); n != len(dressed) || err != nil {
		log.Printf("unexpected error in file password manager: %v", err)
		return ""
	}
	f.reset[hash] = reset
	return url
}

func (f *filePasswordManager) DropReset(user crypto.Token, url, newpassword string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(url)
	if err != nil {
		return false
	}
	hash := crypto.Hasher(decoded)
	f.mu.Lock() // take care unlocking before all exits and before calling Set
	existing, ok := f.passwords[user]
	if !ok || existing >= len(f.hashes) {
		f.mu.Unlock()
		return false
	}
	email := f.hashes[existing].email
	reset, ok := f.reset[hash]
	if !ok {
		f.mu.Unlock()
		return false
	}
	if reset.user != user {
		f.mu.Unlock()
		return false
	}
	delete(f.reset, hash)
	if time.Now().After(reset.expire) {
		f.mu.Unlock()
		return false
	}
	hashedPassword := crypto.Hasher(append(user[:], []byte(newpassword)...))
	delete(f.passwords, user)
	f.mu.Unlock() // final unlock
	if !f.Set(user, hashedPassword, email) {
		f.mu.Lock()
		f.passwords[user] = existing
		f.mu.Unlock()
		return false
	}
	return true
}

func (f *filePasswordManager) Check(user crypto.Token, password crypto.Hash) bool {
	if n, ok := f.passwords[user]; ok {
		if n >= len(f.hashes) {
			log.Printf("unexpected error in file password manager")
			return false
		}
		return password.Equal(f.hashes[n].hash)
	}
	return false
}

func (f *filePasswordManager) Close() {
	f.file.Close()
}

func (f *filePasswordManager) Has(user crypto.Token) bool {
	_, ok := f.passwords[user]
	return ok
}

func (f *filePasswordManager) HasWithEmail(user crypto.Token, email string) bool {
	n, ok := f.passwords[user]
	return ok && f.hashes[n].email == email && email != ""
}

func (f *filePasswordManager) Set(user crypto.Token, password crypto.Hash, email string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	data := []byte{PasswordEntry}
	data = append(data, append(user[:], password[:]...)...)
	data = append(data, []byte(email)...)
	f.file.Seek(0, 2)
	dressed := make([]byte, 0)
	util.PutByteArray(data, &dressed)
	if n, err := f.file.Write(dressed); n != len(dressed) || err != nil {
		log.Printf("unexpected error in file password manager: %v", err)
		return false
	}
	f.hashes = append(f.hashes, hashmail{hash: password, email: email})
	f.passwords[user] = len(f.hashes) - 1
	return true
}

func (f *filePasswordManager) Reset(user crypto.Token, newpassword crypto.Hash) bool {
	f.mu.Lock()
	n, ok := f.passwords[user]
	f.mu.Unlock()
	if !ok || n > len(f.hashes) {
		return false
	}
	return f.Set(user, newpassword, f.hashes[n].email)
}

func NewFilePasswordManager(filename string) PasswordManager {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Fatalf("could not open password manager file: %v", err)
	}
	stat, err := file.Stat()
	if err != nil {
		log.Fatalf("could not stat password manager file: %v", err)
	}
	size := stat.Size()
	// Dangerous: this might be a large file
	if size == 0 {
		return &filePasswordManager{
			file:      *file,
			hashes:    make([]hashmail, 0),
			passwords: make(map[crypto.Token]int),
			reset:     make(map[crypto.Hash]passwordReset),
		}
	}
	data := make([]byte, size)
	if n, err := file.Read(data); n != int(size) {
		log.Fatalf("corrupted password manager file: %v", err)
	}
	//if size%64 != 0 {
	//	log.Fatal("corrupted password manager file: incompatible size")
	//}
	manager := filePasswordManager{
		file:      *file,
		hashes:    make([]hashmail, 0),
		passwords: make(map[crypto.Token]int),
		reset:     make(map[crypto.Hash]passwordReset),
	}
	pos := 0
	var bytes []byte
	for {
		bytes, pos = util.ParseByteArray(data, pos)
		if len(bytes) == 0 {
			break
		}
		if bytes[0] == PasswordEntry {
			token := crypto.Token{}
			copy(token[:], bytes[1:crypto.Size+1])
			hash := crypto.Hash{}
			copy(hash[:], bytes[crypto.Size+1:2*crypto.Size+1])
			email := string(bytes[2*crypto.Size+1:])
			manager.passwords[token] = len(manager.hashes)
			// fmt.Println("em password", token, hash.String())
			manager.hashes = append(manager.hashes, hashmail{hash: hash, email: email})
		}
		if pos == len(data) {
			break
		}
		if pos > len(data) {
			log.Fatal("corrupted password manager file: unexpected end")
		}
	}
	return &manager
}

type PasswordManager interface {
	Check(user crypto.Token, password crypto.Hash) bool
	Set(user crypto.Token, password crypto.Hash, email string) bool
	Reset(user crypto.Token, newpassword crypto.Hash) bool
	Has(user crypto.Token) bool
	HasWithEmail(user crypto.Token, email string) bool
	AddReset(user crypto.Token, email string) string
	DropReset(user crypto.Token, url, newpassword string) bool
	HasReset(url string) (bool, crypto.Token, string)
	Close()
}
