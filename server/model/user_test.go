// SPDX-License-Identifier: AGPL-3.0-or-later

package model_test

import (
	"errors"
	"testing"

	"github.com/asciimoo/hister/server/model"
	"github.com/asciimoo/hister/server/testutil"
)

func TestUpdatePassword(t *testing.T) {
	testutil.InitModel(t)
	testutil.CreateUser(t, "alice")

	if err := model.UpdatePassword("alice", "newpassword123"); err != nil {
		t.Fatal(err)
	}
	if _, err := model.AuthenticateUser("alice", "password123"); !errors.Is(err, model.ErrInvalidPassword) {
		t.Fatalf("old password authentication error = %v, want %v", err, model.ErrInvalidPassword)
	}
	if _, err := model.AuthenticateUser("alice", "newpassword123"); err != nil {
		t.Fatalf("new password authentication failed: %v", err)
	}

	user, err := model.GetUser("alice")
	if err != nil {
		t.Fatal(err)
	}
	if user.Password == "newpassword123" {
		t.Fatal("updated password was stored as plain text")
	}
}

func TestUpdatePasswordUserNotFound(t *testing.T) {
	testutil.InitModel(t)

	err := model.UpdatePassword("missing", "newpassword123")
	if !errors.Is(err, model.ErrUserNotFound) {
		t.Fatalf("UpdatePassword error = %v, want %v", err, model.ErrUserNotFound)
	}
}
