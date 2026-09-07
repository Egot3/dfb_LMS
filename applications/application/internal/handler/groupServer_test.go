package handler_test

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	jwtutils "github.com/egot3/fathom/internal/JWTutils"
	"github.com/egot3/fathom/internal/database/repositories"
	"github.com/egot3/fathom/internal/handler"
	"github.com/egot3/fathom/internal/models"
	testrunner "github.com/egot3/fathom/internal/testRunner"
	"github.com/egot3/fathom/internal/testutils"
	"github.com/egot3/fathom/server"
	"github.com/google/uuid"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"golang.org/x/crypto/bcrypt"
)

func RegisterModels(db *bun.DB) {
	db.RegisterModel((*models.GroupsUsers)(nil))
}

func TestGroupHandler_Post(t *testing.T) {

	t.Run("Create valid group", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.Default())
		do.Provide(i, testrunner.NewManager)

		db := do.MustInvoke[*bun.DB](i)
		RegisterModels(db)

		pswd := rand.Text()
		pswdhash, err := bcrypt.GenerateFromPassword([]byte(pswd), bcrypt.DefaultCost)
		require.NoError(t, err)
		user := models.User{Nickname: rand.Text(), PasswordHash: pswdhash}
		err = db.NewInsert().Model(&user).Scan(t.Context())
		require.NoError(t, err)

		token, err := jwtutils.GenerateToken(user.UUID, true)
		require.NoError(t, err)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/group",
			strings.NewReader(fmt.Sprintf(`{"name": %q}`, rand.Text())),
		)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{
			Name:     "jwt_token",
			Value:    token,
			Path:     "/",
			Expires:  time.Now().Add(jwtutils.JWTTTL),
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		})

		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusCreated, rec.Code, bodyString)
		require.Empty(t, bodyString)
	})
	t.Run("Conflict!", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.Default())
		do.Provide(i, testrunner.NewManager)

		db := do.MustInvoke[*bun.DB](i)
		RegisterModels(db)

		pswd := rand.Text()
		pswdhash, err := bcrypt.GenerateFromPassword([]byte(pswd), bcrypt.DefaultCost)
		require.NoError(t, err)
		user := models.User{Nickname: rand.Text(), PasswordHash: pswdhash}
		err = db.NewInsert().Model(&user).Scan(t.Context())
		require.NoError(t, err)

		token, err := jwtutils.GenerateToken(user.UUID, true)
		require.NoError(t, err)

		name := rand.Text()
		_, err = db.NewInsert().Model(&models.Group{Name: name}).Exec(t.Context())
		require.NoError(t, err)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/group",
			strings.NewReader(fmt.Sprintf(`{"name": %q}`, name)),
		)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{
			Name:     "jwt_token",
			Value:    token,
			Path:     "/",
			Expires:  time.Now().Add(jwtutils.JWTTTL),
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		var groups []models.Group
		err = db.NewSelect().Model(&groups).Scan(t.Context())
		t.Log(groups)

		bodyString := rec.Body.String()
		t.Log(bodyString)
		require.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("With appendants", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.Default())
		do.Provide(i, testrunner.NewManager)

		db := do.MustInvoke[*bun.DB](i)
		RegisterModels(db)

		pswd := rand.Text()
		pswdhash, err := bcrypt.GenerateFromPassword([]byte(pswd), bcrypt.DefaultCost)
		require.NoError(t, err)
		user := models.User{Nickname: rand.Text(), PasswordHash: pswdhash}
		err = db.NewInsert().Model(&user).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		token, err := jwtutils.GenerateToken(user.UUID, true)
		require.NoError(t, err)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/group",
			strings.NewReader(fmt.Sprintf(`{"name": %q, "appendants": [%q]}`, rand.Text(), user.UUID)),
		)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{
			Name:     "jwt_token",
			Value:    token,
			Path:     "/",
			Expires:  time.Now().Add(jwtutils.JWTTTL),
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusCreated, rec.Code, bodyString)
		require.Empty(t, bodyString)

		groupUsers := []models.GroupsUsers{}
		err = db.NewSelect().Model(&groupUsers).
			Where("user_uuid = ?", user.UUID).Scan(t.Context())
		require.NoError(t, err)
	})

	t.Run("Too short nickname", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.Default())
		do.Provide(i, testrunner.NewManager)

		db := do.MustInvoke[*bun.DB](i)
		RegisterModels(db)

		pswd := rand.Text()
		pswdhash, err := bcrypt.GenerateFromPassword([]byte(pswd), bcrypt.DefaultCost)
		require.NoError(t, err)
		user := models.User{Nickname: rand.Text(), PasswordHash: pswdhash}
		err = db.NewInsert().Model(&user).Scan(t.Context())
		require.NoError(t, err)

		token, err := jwtutils.GenerateToken(user.UUID, true)
		require.NoError(t, err)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/group",
			strings.NewReader(fmt.Sprintf(`{"name": %q}`, rand.Text()[0:1])),
		)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{
			Name:     "jwt_token",
			Value:    token,
			Path:     "/",
			Expires:  time.Now().Add(jwtutils.JWTTTL),
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, 422, rec.Code, bodyString)
	})
	t.Run("Too long nickname", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.Default())
		do.Provide(i, testrunner.NewManager)

		db := do.MustInvoke[*bun.DB](i)
		RegisterModels(db)

		pswd := rand.Text()
		pswdhash, err := bcrypt.GenerateFromPassword([]byte(pswd), bcrypt.DefaultCost)
		require.NoError(t, err)
		user := models.User{Nickname: rand.Text(), PasswordHash: pswdhash}
		err = db.NewInsert().Model(&user).Scan(t.Context())
		require.NoError(t, err)

		token, err := jwtutils.GenerateToken(user.UUID, true)
		require.NoError(t, err)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/group",
			strings.NewReader(fmt.Sprintf(`{"name": %q}`, strings.Repeat(rand.Text(), 100))),
		)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{
			Name:     "jwt_token",
			Value:    token,
			Path:     "/",
			Expires:  time.Now().Add(jwtutils.JWTTTL),
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, 422, rec.Code, bodyString)
	})
}

func TestGroupHandler_Delete(t *testing.T) {

	t.Run("Delete group *validly*", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.Default())
		do.Provide(i, testrunner.NewManager)

		db := do.MustInvoke[*bun.DB](i)
		RegisterModels(db)

		pswd := rand.Text()
		pswdhash, err := bcrypt.GenerateFromPassword([]byte(pswd), bcrypt.DefaultCost)
		require.NoError(t, err)
		user := models.User{Nickname: rand.Text(), PasswordHash: pswdhash}
		err = db.NewInsert().Model(&user).Scan(t.Context())
		require.NoError(t, err)

		group := models.Group{Name: rand.Text()}
		err = db.NewInsert().Model(&group).Returning("uuid").
			Scan(t.Context())
		require.NoError(t, err)

		token, err := jwtutils.GenerateToken(user.UUID, true)
		require.NoError(t, err)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodDelete,
			fmt.Sprintf("/api/v1/group/%v", group.UUID),
			nil,
		)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{
			Name:     "jwt_token",
			Value:    token,
			Path:     "/",
			Expires:  time.Now().Add(jwtutils.JWTTTL),
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusNoContent, rec.Code, bodyString)
	})
}

func TestGroupHandler_Patch(t *testing.T) {

	t.Run("Valid name change", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.Default())
		do.Provide(i, testrunner.NewManager)

		db := do.MustInvoke[*bun.DB](i)
		RegisterModels(db)

		pswd := rand.Text()
		pswdhash, err := bcrypt.GenerateFromPassword([]byte(pswd), bcrypt.DefaultCost)
		require.NoError(t, err)
		user := models.User{Nickname: rand.Text(), PasswordHash: pswdhash}
		err = db.NewInsert().Model(&user).Scan(t.Context())
		require.NoError(t, err)

		group := models.Group{Name: rand.Text()}
		err = db.NewInsert().Model(&group).Returning("uuid").
			Scan(t.Context())
		require.NoError(t, err)

		token, err := jwtutils.GenerateToken(user.UUID, true)
		require.NoError(t, err)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPatch,
			fmt.Sprintf("/api/v1/group/%v", group.UUID),
			strings.NewReader(fmt.Sprintf(`{"name": %q}`, rand.Text())),
		)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{
			Name:     "jwt_token",
			Value:    token,
			Path:     "/",
			Expires:  time.Now().Add(jwtutils.JWTTTL),
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusNoContent, rec.Code, bodyString)
	})

	t.Run("Valid no name change", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.Default())
		do.Provide(i, testrunner.NewManager)

		db := do.MustInvoke[*bun.DB](i)
		RegisterModels(db)

		pswd := rand.Text()
		pswdhash, err := bcrypt.GenerateFromPassword([]byte(pswd), bcrypt.DefaultCost)
		require.NoError(t, err)
		user := models.User{Nickname: rand.Text(), PasswordHash: pswdhash}
		err = db.NewInsert().Model(&user).Scan(t.Context())
		require.NoError(t, err)

		group := models.Group{Name: rand.Text()}
		err = db.NewInsert().Model(&group).Returning("uuid").
			Scan(t.Context())
		require.NoError(t, err)

		token, err := jwtutils.GenerateToken(user.UUID, true)
		require.NoError(t, err)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPatch,
			fmt.Sprintf("/api/v1/group/%v", group.UUID),
			nil,
		)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{
			Name:     "jwt_token",
			Value:    token,
			Path:     "/",
			Expires:  time.Now().Add(jwtutils.JWTTTL),
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusNoContent, rec.Code, bodyString)
	})

	t.Run("Not found", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.Default())
		do.Provide(i, testrunner.NewManager)

		db := do.MustInvoke[*bun.DB](i)
		RegisterModels(db)

		pswd := rand.Text()
		pswdhash, err := bcrypt.GenerateFromPassword([]byte(pswd), bcrypt.DefaultCost)
		require.NoError(t, err)
		user := models.User{Nickname: rand.Text(), PasswordHash: pswdhash}
		err = db.NewInsert().Model(&user).Scan(t.Context())
		require.NoError(t, err)

		group := models.Group{Name: rand.Text()}
		err = db.NewInsert().Model(&group).Returning("uuid").
			Scan(t.Context())
		require.NoError(t, err)

		token, err := jwtutils.GenerateToken(user.UUID, true)
		require.NoError(t, err)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPatch,
			fmt.Sprintf("/api/v1/group/%v", uuid.Nil),
			strings.NewReader(fmt.Sprintf(`{"name": %q}`, rand.Text())),
		)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{
			Name:     "jwt_token",
			Value:    token,
			Path:     "/",
			Expires:  time.Now().Add(jwtutils.JWTTTL),
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusNotFound, rec.Code, bodyString)
	})

	t.Run("Bad name changes", func(t *testing.T) {

		t.Run("Too short", func(t *testing.T) {

			i := testutils.NewTestInjector(t,
				repositories.RepositoryPackage,
			)
			do.ProvideValue(i, slog.Default())
			do.Provide(i, testrunner.NewManager)

			db := do.MustInvoke[*bun.DB](i)
			RegisterModels(db)

			pswd := rand.Text()
			pswdhash, err := bcrypt.GenerateFromPassword([]byte(pswd), bcrypt.DefaultCost)
			require.NoError(t, err)
			user := models.User{Nickname: rand.Text(), PasswordHash: pswdhash}
			err = db.NewInsert().Model(&user).Scan(t.Context())
			require.NoError(t, err)

			group := models.Group{Name: rand.Text()}
			err = db.NewInsert().Model(&group).Returning("uuid").
				Scan(t.Context())
			require.NoError(t, err)

			token, err := jwtutils.GenerateToken(user.UUID, true)
			require.NoError(t, err)

			do.Provide(i, handler.NewTestService)
			router, err := server.ChiServer(i)
			require.NoError(t, err)

			req := httptest.NewRequest(
				http.MethodPatch,
				fmt.Sprintf("/api/v1/group/%v", group.UUID),
				strings.NewReader(fmt.Sprintf(`{"name": %q}`, rand.Text()[0:1])),
			)
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(&http.Cookie{
				Name:     "jwt_token",
				Value:    token,
				Path:     "/",
				Expires:  time.Now().Add(jwtutils.JWTTTL),
				HttpOnly: true,
				SameSite: http.SameSiteNoneMode,
				Secure:   true,
			})
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			bodyString := rec.Body.String()
			require.Equal(t, 422, rec.Code, bodyString)
			require.Contains(t, bodyString, "too short")
		})
		t.Run("Too long", func(t *testing.T) {

			i := testutils.NewTestInjector(t,
				repositories.RepositoryPackage,
			)
			do.ProvideValue(i, slog.Default())
			do.Provide(i, testrunner.NewManager)

			db := do.MustInvoke[*bun.DB](i)
			RegisterModels(db)

			pswd := rand.Text()
			pswdhash, err := bcrypt.GenerateFromPassword([]byte(pswd), bcrypt.DefaultCost)
			require.NoError(t, err)
			user := models.User{Nickname: rand.Text(), PasswordHash: pswdhash}
			err = db.NewInsert().Model(&user).Scan(t.Context())
			require.NoError(t, err)

			group := models.Group{Name: rand.Text()}
			err = db.NewInsert().Model(&group).Returning("uuid").
				Scan(t.Context())
			require.NoError(t, err)

			token, err := jwtutils.GenerateToken(user.UUID, true)
			require.NoError(t, err)

			do.Provide(i, handler.NewTestService)
			router, err := server.ChiServer(i)
			require.NoError(t, err)

			req := httptest.NewRequest(
				http.MethodPatch,
				fmt.Sprintf("/api/v1/group/%v", group.UUID),
				strings.NewReader(fmt.Sprintf(`{"name": %q}`, strings.Repeat(rand.Text(), 100))),
			)
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(&http.Cookie{
				Name:     "jwt_token",
				Value:    token,
				Path:     "/",
				Expires:  time.Now().Add(jwtutils.JWTTTL),
				HttpOnly: true,
				SameSite: http.SameSiteNoneMode,
				Secure:   true,
			})
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			bodyString := rec.Body.String()
			require.Equal(t, 422, rec.Code, bodyString)
			require.Contains(t, bodyString, "too long")
		})
	})
}

func TestGroupHandler_Get(t *testing.T) {

	t.Run("Found", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.Default())
		do.Provide(i, testrunner.NewManager)

		db := do.MustInvoke[*bun.DB](i)
		RegisterModels(db)

		pswd := rand.Text()
		pswdhash, err := bcrypt.GenerateFromPassword([]byte(pswd), bcrypt.DefaultCost)
		require.NoError(t, err)
		user := models.User{Nickname: rand.Text(), PasswordHash: pswdhash}
		err = db.NewInsert().Model(&user).Scan(t.Context())
		require.NoError(t, err)

		group := models.Group{Name: rand.Text()}
		err = db.NewInsert().Model(&group).Returning("uuid").
			Scan(t.Context())
		require.NoError(t, err)

		token, err := jwtutils.GenerateToken(user.UUID, true)
		require.NoError(t, err)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodGet,
			fmt.Sprintf("/api/v1/group/%v", group.UUID),
			nil,
		)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{
			Name:     "jwt_token",
			Value:    token,
			Path:     "/",
			Expires:  time.Now().Add(jwtutils.JWTTTL),
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusOK, rec.Code, bodyString)
		require.Contains(t, bodyString, group.UUID.String())
		require.Contains(t, bodyString, group.Name)
	})

	t.Run("Not found", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.Default())
		do.Provide(i, testrunner.NewManager)

		db := do.MustInvoke[*bun.DB](i)
		RegisterModels(db)

		pswd := rand.Text()
		pswdhash, err := bcrypt.GenerateFromPassword([]byte(pswd), bcrypt.DefaultCost)
		require.NoError(t, err)
		user := models.User{Nickname: rand.Text(), PasswordHash: pswdhash}
		err = db.NewInsert().Model(&user).Scan(t.Context())
		require.NoError(t, err)

		group := models.Group{Name: rand.Text()}
		err = db.NewInsert().Model(&group).Returning("uuid").
			Scan(t.Context())
		require.NoError(t, err)

		token, err := jwtutils.GenerateToken(user.UUID, true)
		require.NoError(t, err)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodGet,
			fmt.Sprintf("/api/v1/group/%v", uuid.Nil),
			strings.NewReader(fmt.Sprintf(`{"name": %q}`, rand.Text())),
		)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{
			Name:     "jwt_token",
			Value:    token,
			Path:     "/",
			Expires:  time.Now().Add(jwtutils.JWTTTL),
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusNotFound, rec.Code, bodyString)
	})
}

func TestGroupHandler_List(t *testing.T) {

	t.Run("Size 0", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.Default())
		do.Provide(i, testrunner.NewManager)

		db := do.MustInvoke[*bun.DB](i)
		RegisterModels(db)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodGet,
			fmt.Sprintf("/api/v1/group?size=%d&page=%d", 0, 0),
			nil,
		)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, 422, rec.Code, bodyString)
	})

	t.Run("Valid", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.Default())
		do.Provide(i, testrunner.NewManager)

		db := do.MustInvoke[*bun.DB](i)
		RegisterModels(db)

		pswd := rand.Text()
		pswdhash, err := bcrypt.GenerateFromPassword([]byte(pswd), bcrypt.DefaultCost)
		require.NoError(t, err)
		user := models.User{Nickname: rand.Text(), PasswordHash: pswdhash}
		err = db.NewInsert().Model(&user).Scan(t.Context())
		require.NoError(t, err)

		group := models.Group{Name: rand.Text()}
		err = db.NewInsert().Model(&group).Returning("uuid").
			Scan(t.Context())
		require.NoError(t, err)

		token, err := jwtutils.GenerateToken(user.UUID, true)
		require.NoError(t, err)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodGet,
			fmt.Sprintf("/api/v1/group?size=%d&page=%d", 50, 0),
			nil,
		)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{
			Name:     "jwt_token",
			Value:    token,
			Path:     "/",
			Expires:  time.Now().Add(jwtutils.JWTTTL),
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusOK, rec.Code, bodyString)
		require.Contains(t, bodyString, group.UUID.String())
		require.Contains(t, bodyString, group.Name)
		require.Contains(t, bodyString, "\"total\":1")
	})
}
