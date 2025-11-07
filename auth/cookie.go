package auth

import (
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/freehandle/breeze/crypto"
)

const cookieSessionDuration = 30 * 24 * 60 * 60 // epochs

type CookieStore struct {
	file       *os.File
	handles    map[crypto.Hash]string // hash to cookie
	session    map[string]crypto.Hash // cookie to Hash[handle]
	sessionend map[uint64][]string    //epoch to cookies
	position   map[crypto.Hash]int64  // cookie to position on file
}

func (c *CookieStore) Close() {
	c.file.Close()
}

func (c *CookieStore) Unset(hashHandle crypto.Hash, cookie string) {
	position, ok := c.position[hashHandle]
	if ok {
		bytes := make([]byte, crypto.Size)
		c.file.Seek(position+crypto.Size, 0)
		if n, err := c.file.Write(bytes); n != len(bytes) {
			log.Printf("unexpected error in cookie store: %v", err)
		}
	}
	delete(c.session, cookie)
}

func (c *CookieStore) Clean(epoch uint64) {
	cookies := c.sessionend[epoch]
	for _, cookie := range cookies {
		if hash, ok := c.session[cookie]; ok {
			c.Unset(hash, cookie)
		}
	}
}

func (c *CookieStore) Get(cookie string) (string, bool) {
	hash := c.session[cookie]
	handle, ok := c.handles[hash]
	fmt.Println("COOKIE STORE GET:", cookie, "HANDLE:", handle, "OK:", ok)
	return handle, ok
}

func (c *CookieStore) Set(handle string, cookie string, epoch uint64) bool {
	bytes, err := hex.DecodeString(cookie)
	if err != nil || len(bytes) != crypto.Size {
		return false
	}
	hash := crypto.Hasher([]byte(handle))
	c.handles[hash] = handle
	c.session[cookie] = hash
	fmt.Println("COOKIE STORE GET:", cookie, "HANDLE:")
	epochEnd := epoch + cookieSessionDuration
	c.sessionend[epochEnd] = append(c.sessionend[epochEnd], cookie)
	position, ok := c.position[hash]
	if ok {
		c.file.Seek(position, 0)
	} else {
		bytes = append(hash[:], bytes...) // token + cookie
		c.file.Seek(0, 2)
	}
	if n, err := c.file.Write(bytes); n != len(bytes) {
		log.Printf("unexpected error in cookie store: %v", err)
		return false
	}
	return true
}

func OpenCokieStore(path string) (*CookieStore, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, fmt.Errorf("could not open cookie store file: %v", err)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("could not read cookie store file: %v", err)
	}
	if len(data)%(2*crypto.Size) != 0 {
		return nil, fmt.Errorf("length of cookie store file incompatible: %v", len(data))
	}
	position := 0
	store := &CookieStore{
		file:       file,
		handles:    make(map[crypto.Hash]string),
		session:    make(map[string]crypto.Hash),
		sessionend: make(map[uint64][]string),
		position:   make(map[crypto.Hash]int64),
	}
	epoch := uint64(0)
	for n := 0; n < len(data)/(2*crypto.Size); n++ {
		var hashHandle crypto.Hash
		copy(hashHandle[:], data[2*n*crypto.Size:(2*n+1)*crypto.Size])
		var hash crypto.Hash
		copy(hash[:], data[(2*n+1)*crypto.Size:2*(n+1)*crypto.Size])
		if !hash.Equal(crypto.ZeroHash) && !hashHandle.Equal(crypto.ZeroHash) {
			cookie := hex.EncodeToString(hash[:])
			store.session[cookie] = hashHandle
			store.position[hashHandle] = int64(position)
			endEpoch := epoch + cookieSessionDuration
			store.sessionend[endEpoch] = append(store.sessionend[endEpoch], cookie)
			store.position[hashHandle] = int64(2 * n * crypto.Size)
		}
	}
	return store, nil
}
