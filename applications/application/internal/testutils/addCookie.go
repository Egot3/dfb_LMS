package testutils

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	jwtutils "github.com/egot3/fathom/internal/JWTutils"
	"github.com/stretchr/testify/require"
)

func AddTeacherCookie(t *testing.T, req *http.Request) {
	t.Helper()

	token, err := jwtutils.GenerateToken(uuid.Nil, true)
	require.NoError(t, err)

	req.AddCookie(&http.Cookie{
		Name:     "jwt_token",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(jwtutils.JWTTTL),
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	})
}
