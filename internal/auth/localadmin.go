// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/panset/hourglass/internal/store"
)

// LocalAdminUsername is the permanent local account's username.
const LocalAdminUsername = "admin"

// generatedPasswordLen is the length of a generated initial password.
const generatedPasswordLen = 24

// EnsureLocalAdmin creates the local admin (auth_source local, role admin,
// active, must_change_password) only if it does not exist. The password is
// initialPassword (HOURGLASS_ADMIN_PASSWORD), or, when that is empty, a
// generated 24-character one printed once to log at creation. If the row
// exists, initialPassword is ignored (rules list, AUTH-7).
func EnsureLocalAdmin(ctx context.Context, st *store.Store, initialPassword string, log *slog.Logger) (store.User, error) {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	u, err := st.GetUserByUsername(ctx, LocalAdminUsername)
	if err == nil && u.AuthSource == store.AuthLocal {
		return u, nil
	}
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return store.User{}, err
	}
	if err == nil {
		// OIDC users never have a username, so this is a corrupted database.
		return store.User{}, errors.New("auth: a non-local user is named admin")
	}
	pw, generated := initialPassword, false
	if pw == "" {
		if pw, err = GeneratePassword(generatedPasswordLen); err != nil {
			return store.User{}, err
		}
		generated = true
	} else if err := ValidatePassword(pw); err != nil {
		return store.User{}, errors.New("auth: the initial admin password is invalid: " + err.Error())
	}
	hash, err := HashPassword(pw)
	if err != nil {
		return store.User{}, err
	}
	u, err = st.CreateLocalAdmin(ctx, LocalAdminUsername, hash, true, time.Now())
	if err != nil {
		return store.User{}, err
	}
	if generated {
		// The one deliberate exception to "no secrets in logs" (spec rules list):
		// printed exactly once, at creation, and it must be changed at first sign-in.
		log.Warn("created the local admin account with a generated password; it must be changed at first sign-in",
			"username", LocalAdminUsername, "password", pw)
	} else {
		log.Info("created the local admin account with the configured initial password", "username", LocalAdminUsername)
	}
	return u, nil
}
