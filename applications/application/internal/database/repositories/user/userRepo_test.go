package user_test

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"testing"

	"github.com/egot3/fathom/internal/database/repositories/user"
	"github.com/egot3/fathom/internal/models"
	"github.com/egot3/fathom/internal/testutils"
	"github.com/google/uuid"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"golang.org/x/crypto/bcrypt"
)

func NewInjectorWithUserRepo(t *testing.T) do.Injector {
	t.Helper()

	i := testutils.NewTestInjector(t)

	do.Provide(i, user.NewUserRepository)

	return i
}

func TestUser_Register(t *testing.T) {

	i := NewInjectorWithUserRepo(t)

	r := do.MustInvoke[user.UserRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(rand.Text()), bcrypt.DefaultCost)
	require.NoError(t, err)

	name := rand.Text()
	user, err := r.Register(t.Context(), name, passwordHash)
	require.NoError(t, err) //there is just nothing to test

	userRetrieved := models.User{UUID: user.UUID}
	err = db.NewSelect().Model(&userRetrieved).WherePK().Scan(t.Context())
	require.NoError(t, err)

	require.Equal(t, name, userRetrieved.Nickname)
	require.Equal(t, passwordHash, userRetrieved.PasswordHash)
	require.Equal(t, false, userRetrieved.IsTeacher)
}

func TestUser_Login(t *testing.T) {

	i := NewInjectorWithUserRepo(t)

	r := do.MustInvoke[user.UserRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	password := []byte(rand.Text())
	name := rand.Text()

	pswdHash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	require.NoError(t, err)
	var origUser models.User = models.User{Nickname: name, PasswordHash: pswdHash}
	err = db.NewInsert().
		Model(&origUser).
		Scan(t.Context())
	require.NoError(t, err)

	testCases := []struct {
		desc       string
		nickname   string
		password   []byte
		ExpectFail bool
	}{
		{
			desc:       "Valid login",
			nickname:   name,
			password:   password,
			ExpectFail: false,
		},
		{
			desc:       "Bad nickname",
			nickname:   "",
			password:   password,
			ExpectFail: true,
		},
		{
			desc:       "Bad password",
			nickname:   name,
			password:   []byte("12313"),
			ExpectFail: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {

			user, err := r.Login(t.Context(), tC.nickname, tC.password)

			if tC.ExpectFail {
				require.Error(t, err)
				require.ErrorIs(t, err, sql.ErrNoRows)
			} else {
				require.NoError(t, err)
				require.Equal(t, origUser.UUID, user.UUID)
			}
		})
	}
}

func TestUser_User(t *testing.T) {

	i := NewInjectorWithUserRepo(t)

	r := do.MustInvoke[user.UserRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	passwordHash := []byte(rand.Text())
	name := rand.Text()

	user := models.User{Nickname: name, PasswordHash: passwordHash}
	err := db.NewInsert().
		Model(&user).
		Scan(t.Context())
	require.NoError(t, err)

	testCases := []struct {
		desc           string
		uuid           uuid.UUID
		ExpectNotFound bool
	}{
		{
			desc:           "Valid user",
			uuid:           user.UUID,
			ExpectNotFound: false,
		},
		{
			desc:           "No user",
			uuid:           uuid.Nil,
			ExpectNotFound: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {

			userR, err := r.User(t.Context(), tC.uuid)

			if tC.ExpectNotFound {
				require.True(t, errors.Is(err, sql.ErrNoRows))
			} else {
				require.Equal(t, models.User{
					Nickname:     user.Nickname,
					UUID:         user.UUID,
					IsTeacher:    user.IsTeacher,
					PasswordHash: nil, //safety
				}, userR)
			}
		})
	}
}

func TestUser_Exists(t *testing.T) { // usage: JWT

	i := NewInjectorWithUserRepo(t)

	r := do.MustInvoke[user.UserRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(rand.Text()), bcrypt.DefaultCost)
	require.NoError(t, err)
	name := rand.Text()

	user := models.User{Nickname: name, PasswordHash: passwordHash}
	err = db.NewInsert().
		Model(&user).
		Scan(t.Context())
	require.NoError(t, err)

	testCases := []struct {
		desc           string
		uuid           uuid.UUID
		ExpectNotFound bool
	}{
		{
			desc:           "Found user",
			uuid:           user.UUID,
			ExpectNotFound: false,
		},
		{
			desc:           "No user",
			uuid:           uuid.Nil,
			ExpectNotFound: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {

			found, err := r.Exists(t.Context(), tC.uuid)
			require.NoError(t, err)

			if tC.ExpectNotFound {
				require.False(t, found)
			} else {
				require.True(t, found)
			}
		})
	}
}

func TestUser_Delete(t *testing.T) {

	i := NewInjectorWithUserRepo(t)

	r := do.MustInvoke[user.UserRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(rand.Text()), bcrypt.DefaultCost)
	require.NoError(t, err)

	user := models.User{Nickname: rand.Text(), PasswordHash: passwordHash}
	err = db.NewInsert().
		Model(&user).
		Scan(t.Context())
	require.NoError(t, err)

	testCases := []struct {
		desc           string
		uuid           uuid.UUID
		ExpectNotFound bool
	}{
		{
			desc:           "Deleted user",
			uuid:           user.UUID,
			ExpectNotFound: false,
		},
		{
			desc:           "No user",
			uuid:           uuid.Nil,
			ExpectNotFound: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {

			err := r.DeleteUser(t.Context(), tC.uuid)

			if tC.ExpectNotFound {
				require.Error(t, err)
				require.True(t, errors.Is(err, sql.ErrNoRows))
				return
			}
			require.NoError(t, err)

			f, err := db.NewSelect().Model(&user).WherePK().Exists(t.Context())
			require.NoError(t, err)
			require.False(t, f)
		})
	}
}

func TestUser_IsTeacher(t *testing.T) { // usage: JWT

	i := NewInjectorWithUserRepo(t)

	r := do.MustInvoke[user.UserRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(rand.Text()), bcrypt.DefaultCost)
	require.NoError(t, err)

	teacher := models.User{Nickname: rand.Text(), PasswordHash: passwordHash, IsTeacher: true}
	err = db.NewInsert().
		Model(&teacher).
		Scan(t.Context())
	require.NoError(t, err)

	pupil := models.User{Nickname: rand.Text(), PasswordHash: passwordHash, IsTeacher: false}
	err = db.NewInsert().
		Model(&pupil).
		Scan(t.Context())
	require.NoError(t, err)

	testCases := []struct {
		desc             string
		uuid             uuid.UUID
		ExpectNotTeacher bool
	}{
		{
			desc:             "Teacher",
			uuid:             teacher.UUID,
			ExpectNotTeacher: false,
		},
		{
			desc:             "No user",
			uuid:             uuid.Nil,
			ExpectNotTeacher: true,
		},
		{
			desc:             "Pupil",
			uuid:             pupil.UUID,
			ExpectNotTeacher: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {

			isTeacher, err := r.IsTeacher(t.Context(), tC.uuid)
			require.NoError(t, err)

			if tC.ExpectNotTeacher {
				require.False(t, isTeacher)
			} else {
				require.True(t, isTeacher)
			}

		})
	}
}

func TestUser_Update(t *testing.T) {

	i := NewInjectorWithUserRepo(t)

	r := do.MustInvoke[user.UserRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(rand.Text()), bcrypt.DefaultCost)
	require.NoError(t, err)
	name := rand.Text()

	_, err = db.NewInsert().
		Model(&models.User{Nickname: name, PasswordHash: passwordHash}).
		Exec(t.Context())
	require.NoError(t, err)

	t.Run("Not found", func(t *testing.T) {

	})

	testCases := []struct {
		desc             string
		uuid             uuid.UUID
		nicknameUpdated  bool
		passwordUpdated  bool
		isTeacherUpdated bool
	}{
		{
			desc:             "Update All",
			nicknameUpdated:  true,
			passwordUpdated:  true,
			isTeacherUpdated: true,
		},
		{
			desc:             "Update nickname",
			nicknameUpdated:  true,
			passwordUpdated:  false,
			isTeacherUpdated: false,
		},
		{
			desc:             "Update password",
			nicknameUpdated:  false,
			passwordUpdated:  true,
			isTeacherUpdated: false,
		},
		{
			desc:             "Update isTeacher",
			nicknameUpdated:  false,
			passwordUpdated:  false,
			isTeacherUpdated: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {

			user := models.User{Nickname: rand.Text(), PasswordHash: passwordHash}
			err = db.NewInsert().
				Model(&user).
				Scan(t.Context())
			require.NoError(t, err)

			var nickname *string = nil
			if tC.nicknameUpdated {
				n := rand.Text()
				nickname = &n
			}
			var password []byte = nil
			if tC.passwordUpdated {
				passwordHash, err := bcrypt.GenerateFromPassword([]byte(rand.Text()), bcrypt.DefaultCost)
				require.NoError(t, err)
				password = append(password, passwordHash...)
			}
			var isTeacher *bool = nil
			if tC.isTeacherUpdated {
				t := true
				isTeacher = &t
			}

			err = r.UpdateUser(t.Context(), models.PatchUser{
				UUID:         user.UUID,
				Nickname:     nickname,
				PasswordHash: password,
				IsTeacher:    isTeacher,
			})
			require.NoError(t, err)
		})
	}
}
