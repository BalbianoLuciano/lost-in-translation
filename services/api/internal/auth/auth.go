// Package auth verifica la identidad de quien llama a la API.
//
// En producción el front obtiene un ID token de Firebase Authentication y lo
// manda como Bearer; acá sólo se verifica. En desarrollo local se puede usar
// DevVerifier para no depender de un proyecto de Firebase.
package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	firebase "firebase.google.com/go/v4"
	fbauth "firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

var ErrInvalidToken = errors.New("token inválido")

type Identity struct {
	UID   string
	Email string
	Name  string
}

type Verifier interface {
	Verify(ctx context.Context, token string) (Identity, error)
}

// FirebaseVerifier verifica ID tokens contra las claves públicas de Firebase.
type FirebaseVerifier struct {
	client *fbauth.Client
}

func NewFirebaseVerifier(ctx context.Context, projectID, credentialsJSON string) (*FirebaseVerifier, error) {
	opt := option.WithoutAuthentication()
	if credentialsJSON != "" {
		opt = option.WithAuthCredentialsJSON(option.ServiceAccount, []byte(credentialsJSON))
	}
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID}, opt)
	if err != nil {
		return nil, err
	}
	client, err := app.Auth(ctx)
	if err != nil {
		return nil, err
	}
	return &FirebaseVerifier{client: client}, nil
}

func (v *FirebaseVerifier) Verify(ctx context.Context, token string) (Identity, error) {
	t, err := v.client.VerifyIDToken(ctx, token)
	if err != nil {
		return Identity{}, errors.Join(ErrInvalidToken, err)
	}
	email, _ := t.Claims["email"].(string)
	name, _ := t.Claims["name"].(string)
	return Identity{UID: t.UID, Email: email, Name: name}, nil
}

// DevVerifier acepta tokens con forma "dev:<uid>:<email>". Nunca en producción:
// config.Load lo impide.
type DevVerifier struct{}

func (DevVerifier) Verify(_ context.Context, token string) (Identity, error) {
	parts := strings.SplitN(token, ":", 3)
	if len(parts) != 3 || parts[0] != "dev" || parts[1] == "" || parts[2] == "" {
		return Identity{}, ErrInvalidToken
	}
	return Identity{UID: parts[1], Email: parts[2], Name: parts[1]}, nil
}

type ctxKey struct{}

func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

func FromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	return id, ok
}

// Middleware exige un Bearer token válido y deja la identidad en el contexto.
func Middleware(v Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || token == "" {
				unauthorized(w)
				return
			}
			id, err := v.Verify(r.Context(), token)
			if err != nil {
				unauthorized(w)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
		})
	}
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
}
