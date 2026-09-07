package group_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	mrand "math/rand/v2"
	"testing"

	"github.com/egot3/fathom/internal/carefulness"
	"github.com/egot3/fathom/internal/database/repositories/group"
	"github.com/egot3/fathom/internal/models"
	"github.com/egot3/fathom/internal/testutils"
	"github.com/google/uuid"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func NewInjectorWithGroupRepo(t testing.TB) do.Injector {
	t.Helper()

	i := testutils.NewTestInjector(t)

	do.Provide(i, group.NewGroupRepository)

	return i
}

func RegisterModels(db *bun.DB) {
	db.RegisterModel((*models.GroupsUsers)(nil))
}

func TestGroup_Creation(t *testing.T) {

	i := NewInjectorWithGroupRepo(t)

	r := do.MustInvoke[group.GroupRepository](i)
	db := do.MustInvoke[*bun.DB](i)
	RegisterModels(db)

	t.Run("New group", func(t *testing.T) {

		name := rand.Text()
		_, err := r.NewGroup(t.Context(), name)
		require.NoError(t, err)

		var group models.Group
		err = db.NewSelect().Model(&group).Where("name = ?", name).Scan(t.Context())
		require.NoError(t, err)
		require.Equal(t, name, group.Name)
	})

	t.Run("Existing group", func(t *testing.T) {

		name := rand.Text()
		_, err := r.NewGroup(t.Context(), name)
		require.NoError(t, err)

		var group models.Group
		err = db.NewSelect().Model(&group).Where("name = ?", name).Scan(t.Context())
		require.NoError(t, err)
		require.Equal(t, name, group.Name)

		_, err = r.NewGroup(t.Context(), name)
		require.Error(t, err)
		require.ErrorIs(t, err, carefulness.ErrConflict)
	})
}

func TestGroup_Deletion(t *testing.T) {

	i := NewInjectorWithGroupRepo(t)

	r := do.MustInvoke[group.GroupRepository](i)
	db := do.MustInvoke[*bun.DB](i)
	RegisterModels(db)

	name := rand.Text()
	_, err := db.NewInsert().Model(&models.Group{Name: name}).Exec(t.Context())
	require.NoError(t, err)

	group := models.Group{}
	err = db.NewSelect().Model(&group).Where("name = ?", name).Scan(t.Context())

	t.Run("Existing group", func(t *testing.T) {

		err := r.DeleteGroup(t.Context(), group.UUID)
		require.NoError(t, err)

		err = db.NewSelect().Model(&group).WherePK().Scan(t.Context())
		require.Error(t, err)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("Non-existing group", func(t *testing.T) {

		err := r.DeleteGroup(t.Context(), uuid.Nil)
		require.Error(t, err)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

}

func TestGroup_Get(t *testing.T) {

	i := NewInjectorWithGroupRepo(t)

	r := do.MustInvoke[group.GroupRepository](i)
	db := do.MustInvoke[*bun.DB](i)
	RegisterModels(db)

	name := rand.Text()
	_, err := db.NewInsert().Model(&models.Group{Name: name}).Exec(t.Context())
	require.NoError(t, err)

	group := models.Group{}
	err = db.NewSelect().Model(&group).Where("name = ?", name).Scan(t.Context())

	t.Run("Existing group", func(t *testing.T) {

		g, err := r.Group(t.Context(), group.UUID)
		require.NoError(t, err)

		require.Equal(t, group.UUID, g.UUID)
		require.Equal(t, group.Name, g.Name)
		require.Nil(t, g.Users) //created empty
	})

	t.Run("Non-existing group", func(t *testing.T) {

		_, err := r.Group(t.Context(), uuid.Nil)
		require.Error(t, err)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})
}

func TestGroup_Update(t *testing.T) {

	i := NewInjectorWithGroupRepo(t)

	r := do.MustInvoke[group.GroupRepository](i)
	db := do.MustInvoke[*bun.DB](i)
	RegisterModels(db)

	name := rand.Text()
	groupUUID := uuid.UUID{}
	err := db.NewInsert().Model(&models.Group{Name: name}).Returning("uuid").Scan(t.Context(), &groupUUID)
	require.NoError(t, err)

	t.Run("Non-existant quiz", func(t *testing.T) {

		err := r.UpdateGroup(t.Context(), uuid.Nil, "")
		require.Error(t, err)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("Existant quiz", func(t *testing.T) {

		name := rand.Text()
		err := r.UpdateGroup(t.Context(), groupUUID, name)
		require.NoError(t, err)

		var newName string
		err = db.NewSelect().Model(&models.Group{UUID: groupUUID}).WherePK().Column("name").Scan(t.Context(), &newName)
		require.NoError(t, err)
		require.Equal(t, name, newName)
	})
}

func TestGroup_Append_users(t *testing.T) {

	i := NewInjectorWithGroupRepo(t)

	r := do.MustInvoke[group.GroupRepository](i)
	db := do.MustInvoke[*bun.DB](i)
	RegisterModels(db)

	name := rand.Text()
	groupUUID := uuid.UUID{}
	err := db.NewInsert().Model(&models.Group{Name: name}).Returning("uuid").Scan(t.Context(), &groupUUID)
	require.NoError(t, err)

	var uuids uuid.UUIDs
	for i := 0; i < mrand.IntN(8)+1; i++ {
		var userUUID uuid.UUID
		err = db.NewInsert().Model(&models.User{Nickname: rand.Text(), PasswordHash: []byte{}}).
			Returning("uuid").
			Scan(t.Context(), &userUUID)
		require.NoError(t, err)
		uuids = append(uuids, userUUID)
	}

	t.Run("Append non-existant user", func(t *testing.T) {

		err := r.AppendUsers(t.Context(), groupUUID, uuid.UUIDs{uuid.Nil})
		require.Error(t, err)
	})

	t.Run("Append partially existing users", func(t *testing.T) {

		err := r.AppendUsers(t.Context(), groupUUID, append(uuids, uuid.Nil))
		require.Error(t, err)
	})

	t.Run("Append users(duh)", func(t *testing.T) {

		err := r.AppendUsers(t.Context(), groupUUID, uuids)
		require.NoError(t, err)
	})
}

func BenchmarkGroup_Append_users(b *testing.B) {
	b.Run("Benchmark 5 appendants", func(b *testing.B) {
		i := NewInjectorWithGroupRepo(b)

		r := do.MustInvoke[group.GroupRepository](i)
		db := do.MustInvoke[*bun.DB](i)
		RegisterModels(db)

		name := rand.Text()
		groupUUID := uuid.UUID{}
		err := db.NewInsert().Model(&models.Group{Name: name}).Returning("uuid").Scan(b.Context(), &groupUUID)
		require.NoError(b, err)

		var uuids uuid.UUIDs
		for range 5 {
			var userUUID uuid.UUID
			err = db.NewInsert().Model(&models.User{Nickname: rand.Text(), PasswordHash: []byte{}}).
				Returning("uuid").
				Scan(b.Context(), &userUUID)
			require.NoError(b, err)
			uuids = append(uuids, userUUID)
		}

		b.ResetTimer()

		for b.Loop() {
			b.StopTimer()
			_, err := db.NewTruncateTable().Table("groups_users").Exec(b.Context())
			require.NoError(b, err)
			b.StartTimer()

			err = r.AppendUsers(b.Context(), groupUUID, uuids)
			require.NoError(b, err)
		}
	})

	b.Run("Benchmark 50 appendants", func(b *testing.B) {
		i := NewInjectorWithGroupRepo(b)

		r := do.MustInvoke[group.GroupRepository](i)
		db := do.MustInvoke[*bun.DB](i)
		RegisterModels(db)

		name := rand.Text()
		groupUUID := uuid.UUID{}
		err := db.NewInsert().Model(&models.Group{Name: name}).Returning("uuid").Scan(b.Context(), &groupUUID)
		require.NoError(b, err)

		var uuids uuid.UUIDs
		for range 50 {
			var userUUID uuid.UUID
			err = db.NewInsert().Model(&models.User{Nickname: rand.Text(), PasswordHash: []byte{}}).
				Returning("uuid").
				Scan(b.Context(), &userUUID)
			require.NoError(b, err)
			uuids = append(uuids, userUUID)
		}

		b.ResetTimer()

		for b.Loop() {
			b.StopTimer()
			_, err := db.NewTruncateTable().Table("groups_users").Exec(b.Context())
			require.NoError(b, err)
			b.StartTimer()

			err = r.AppendUsers(b.Context(), groupUUID, uuids)
			require.NoError(b, err)
		}
	})

	b.Run("Benchmark 500 appendants", func(b *testing.B) {
		i := NewInjectorWithGroupRepo(b)

		r := do.MustInvoke[group.GroupRepository](i)
		db := do.MustInvoke[*bun.DB](i)
		RegisterModels(db)

		name := rand.Text()
		groupUUID := uuid.UUID{}
		err := db.NewInsert().Model(&models.Group{Name: name}).Returning("uuid").Scan(b.Context(), &groupUUID)
		require.NoError(b, err)

		var uuids uuid.UUIDs
		for range 500 {
			var userUUID uuid.UUID
			err = db.NewInsert().Model(&models.User{Nickname: rand.Text(), PasswordHash: []byte{}}).
				Returning("uuid").
				Scan(b.Context(), &userUUID)
			require.NoError(b, err)
			uuids = append(uuids, userUUID)
		}

		b.ResetTimer()

		for b.Loop() {
			b.StopTimer()
			_, err := db.NewTruncateTable().Table("groups_users").Exec(b.Context())
			require.NoError(b, err)
			b.StartTimer()

			err = r.AppendUsers(b.Context(), groupUUID, uuids)
			require.NoError(b, err)
		}
	})
}

func TestGroup_Remove_users(t *testing.T) {

	i := NewInjectorWithGroupRepo(t)

	r := do.MustInvoke[group.GroupRepository](i)
	db := do.MustInvoke[*bun.DB](i)
	RegisterModels(db)

	name := rand.Text()
	groupUUID := uuid.UUID{}
	err := db.NewInsert().Model(&models.Group{Name: name}).Returning("uuid").Scan(t.Context(), &groupUUID)
	require.NoError(t, err)

	var users []models.User
	for i := 0; i < mrand.IntN(8)+1; i++ {
		user := models.User{Nickname: rand.Text(), PasswordHash: []byte{}}
		users = append(users, user)
	}
	var uuids uuid.UUIDs
	err = db.NewInsert().Model(&users).
		Returning("uuid").
		Scan(t.Context(), &uuids)
	require.NoError(t, err)

	var usersGroup []models.GroupsUsers
	for _, userUUID := range uuids {
		usersGroup = append(usersGroup, models.GroupsUsers{UserUUID: userUUID, GroupUUID: groupUUID})
	}
	_, err = db.NewInsert().Model(&usersGroup).
		Exec(t.Context())
	require.NoError(t, err)

	t.Run("Remove non-existant user", func(t *testing.T) {

		err := r.RemoveUsers(t.Context(), groupUUID, uuid.UUIDs{uuid.Nil})
		require.Error(t, err)
	})

	t.Run("Remove partially existing users", func(t *testing.T) {

		err := r.RemoveUsers(t.Context(), groupUUID, append(uuids, uuid.Nil))
		require.Error(t, err)
	})

	t.Run("Remove users(duh)", func(t *testing.T) {

		err := r.RemoveUsers(t.Context(), groupUUID, uuids)
		require.NoError(t, err)
	})
}

func TestGroups_Is_in(t *testing.T) {

	i := NewInjectorWithGroupRepo(t)

	r := do.MustInvoke[group.GroupRepository](i)
	db := do.MustInvoke[*bun.DB](i)
	RegisterModels(db)

	name := rand.Text()
	groupUUID := uuid.UUID{}
	err := db.NewInsert().Model(&models.Group{Name: name}).Returning("uuid").Scan(t.Context(), &groupUUID)
	require.NoError(t, err)

	var userUUID uuid.UUID
	err = db.NewInsert().Model(&models.User{Nickname: rand.Text(), PasswordHash: []byte{}}).
		Returning("uuid").
		Scan(t.Context(), &userUUID)
	require.NoError(t, err)

	_, err = db.NewInsert().Model(&models.GroupsUsers{GroupUUID: groupUUID, UserUUID: userUUID}).
		Exec(t.Context())
	require.NoError(t, err)

	testCases := []struct {
		desc        string
		uuid        uuid.UUID
		expectNotIn bool
	}{
		{
			desc:        "In",
			uuid:        userUUID,
			expectNotIn: false,
		},
		{
			desc:        "Not in",
			uuid:        uuid.Nil,
			expectNotIn: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			is, err := r.IsInGroup(context.Background(), groupUUID, tC.uuid)
			require.NoError(t, err)
			if tC.expectNotIn {
				require.False(t, is)
			} else {
				require.True(t, is)
			}
		})
	}
}
