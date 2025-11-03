package auth

import (
    "context"
    "errors"
    "net/http"

    "golang.org/x/crypto/bcrypt"
)

type UserStore interface {
    HasUser(username string) bool
    CheckPassword(username, plain string) bool
}

type InMemoryUserStore struct {
    // username -> bcrypt hash
    hashes map[string][]byte
}

func NewInMemoryUserStore() *InMemoryUserStore {
    return &InMemoryUserStore{hashes: make(map[string][]byte)}
}

func (s *InMemoryUserStore) HasUser(username string) bool {
    _, ok := s.hashes[username]
    return ok
}

func (s *InMemoryUserStore) AddUserPlain(username, password string) error {
    if username == "" {
        return errors.New("username empty")
    }
    if password == "" {
        return errors.New("password empty")
    }
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return err
    }
    s.hashes[username] = hash
    return nil
}

func (s *InMemoryUserStore) AddUserHash(username string, bcryptHash []byte) error {
    if username == "" {
        return errors.New("username empty")
    }
    if len(bcryptHash) == 0 {
        return errors.New("hash empty")
    }
    s.hashes[username] = bcryptHash
    return nil
}

func (s *InMemoryUserStore) CheckPassword(username, plain string) bool {
    hash, ok := s.hashes[username]
    if !ok {
        return false
    }
    if err := bcrypt.CompareHashAndPassword(hash, []byte(plain)); err != nil {
        return false
    }
    return true
}

func BasicAuthMiddleware(store UserStore, realm string, next http.Handler) http.Handler {
    if realm == "" {
        realm = "Restricted"
    }
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        username, password, ok := r.BasicAuth()
        if !ok {
            unauthorized(w, realm)
            return
        }
        if !store.HasUser(username) {
            unauthorized(w, realm)
            return
        }
        if !store.CheckPassword(username, password) {
            unauthorized(w, realm)
            return
        }
        // attach username to context for later use
        ctx := context.WithValue(r.Context(), userKey, username)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func unauthorized(w http.ResponseWriter, realm string) {
    // protect against header smuggling; fixed compare of string is unnecessary here
    w.Header().Set("WWW-Authenticate", "Basic realm=\""+realm+"\"")
    http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
}

type contextKey string

const userKey contextKey = "auth.user"

func UsernameFromRequest(r *http.Request) (string, bool) {
    v := r.Context().Value(userKey)
    s, ok := v.(string)
    return s, ok
}


