// Copyright 2020 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"testing"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"

	"gogs.io/gogs/internal/errutil"
)

func Test_users(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	tables := []interface{}{new(User), new(EmailAddress)}
	db := &users{
		DB: initTestDB(t, "users", tables...),
	}

	for _, tc := range []struct {
		name string
		test func(*testing.T, *users)
	}{
		{"Authenticate", test_users_Authenticate},
		{"Create", test_users_Create},
		// {"GetByEmail", test_users_GetByEmail},
		// {"GetByID", test_users_GetByID},
		// {"GetByUsername", test_users_GetByUsername},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(func() {
				err := clearTables(t, db.DB, tables...)
				if err != nil {
					t.Fatal(err)
				}
			})
			tc.test(t, db)
		})
	}
}

// TODO: Only local account is tested, tests for external account will be added
//  along with addressing https://github.com/gogs/gogs/issues/6115.
func test_users_Authenticate(t *testing.T, db *users) {
	password := "pa$$word"
	alice, err := db.Create(CreateUserOpts{
		Name:     "alice",
		Email:    "alice@example.com",
		Password: password,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("user not found", func(t *testing.T) {
		_, err := db.Authenticate("bob", password, -1)
		expErr := ErrUserNotExist{args: map[string]interface{}{"login": "bob"}}
		assert.Equal(t, expErr, err)
	})

	t.Run("invalid password", func(t *testing.T) {
		_, err := db.Authenticate(alice.Name, "bad_password", -1)
		expErr := ErrUserNotExist{args: map[string]interface{}{"userID": alice.ID, "name": alice.Name}}
		assert.Equal(t, expErr, err)
	})

	t.Run("via email and password", func(t *testing.T) {
		user, err := db.Authenticate(alice.Email, password, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, alice.Name, user.Name)
	})

	t.Run("via username and password", func(t *testing.T) {
		user, err := db.Authenticate(alice.Name, password, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, alice.Name, user.Name)
	})
}

func test_users_Create(t *testing.T, db *users) {
	alice, err := db.Create(CreateUserOpts{
		Name:      "alice",
		Email:     "alice@example.com",
		Activated: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("name not allowed", func(t *testing.T) {
		_, err := db.Create(CreateUserOpts{
			Name: "-",
		})
		expErr := ErrNameNotAllowed{args: errutil.Args{"reason": "reserved", "name": "-"}}
		assert.Equal(t, expErr, err)
	})

	t.Run("name already exists", func(t *testing.T) {
		_, err := db.Create(CreateUserOpts{
			Name: alice.Name,
		})
		expErr := ErrUserAlreadyExist{args: errutil.Args{"name": alice.Name}}
		assert.Equal(t, expErr, err)
	})

	t.Run("email already exists", func(t *testing.T) {
		_, err := db.Create(CreateUserOpts{
			Name:  "bob",
			Email: alice.Email,
		})
		expErr := ErrEmailAlreadyUsed{args: errutil.Args{"email": alice.Email}}
		assert.Equal(t, expErr, err)
	})

	user, err := db.GetByUsername(alice.Name)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, gorm.NowFunc().Format(time.RFC3339), user.Created.Format(time.RFC3339))
	assert.Equal(t, gorm.NowFunc().Format(time.RFC3339), user.Updated.Format(time.RFC3339))
}
