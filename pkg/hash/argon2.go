// Package hash turns passwords into argon2id hashes and checks them again.
//
// The cost parameters travel inside the hash string rather than living in
// constants here. That is what lets the cost be raised later without a password
// migration: an old hash is still verified with the parameters it was made with,
// and only new hashes use the new ones.
package hash

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"golang.org/x/crypto/argon2"
)

// ErrInvalidHash means the stored string is not an argon2id hash this package
// can read. It is a corrupt-data error, not a wrong-password one, and callers
// must not treat it as a failed login.
var ErrInvalidHash = errors.New("hash: malformed argon2id hash")

// Params are the cost settings used for new hashes.
//
// Memory and Time follow the OWASP guidance for argon2id (64 MiB, 3 passes).
// Parallelism tracks the machine rather than being pinned to a number, since a
// hash made anywhere still carries its own value for verification.
type Params struct {
	Memory      uint32 // KiB
	Time        uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultParams is what Password uses. Raising these affects new passwords only.
var DefaultParams = Params{
	Memory:      64 * 1024,
	Time:        3,
	Parallelism: uint8(min(runtime.NumCPU(), 4)),
	SaltLength:  16,
	KeyLength:   32,
}

// DummyHash is a valid hash of a fixed string. Login verifies against it when no
// user matches the address, so that a missing account and a wrong password take
// the same time to answer. Without it the response time reveals which addresses
// are registered, whatever the error message says.
//
// It is a constant rather than something computed at init so that process start
// does not pay for an argon2 run.
const DummyHash = "$argon2id$v=19$m=65536,t=3,p=2$ZGVhZGJlZWZkZWFkYmVlZg$" +
	"jLDG3mIKyCl2CaeHwS3uA7PwctN0EwUH44aJ7dDv+8Y"

// Password hashes a plaintext password with a fresh random salt. Hashing the
// same password twice gives different strings, which is the salt doing its job.
func Password(plain string) (string, error) {
	return passwordWith(plain, DefaultParams)
}

func passwordWith(plain string, p Params) (string, error) {
	salt := make([]byte, p.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("hash: read salt: %w", err)
	}

	key := argon2.IDKey([]byte(plain), salt, p.Time, p.Memory, p.Parallelism, p.KeyLength)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Time, p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Verify reports whether plain is the password behind encoded.
//
// A false result with a nil error means the password is wrong. An error means
// the stored hash could not be read at all, which is a different problem and
// deserves a different response than "wrong password".
func Verify(plain, encoded string) (bool, error) {
	p, salt, want, err := decode(encoded)
	if err != nil {
		return false, err
	}

	got := argon2.IDKey([]byte(plain), salt, p.Time, p.Memory, p.Parallelism, p.KeyLength)

	// Constant time: a byte-by-byte comparison that returns early leaks how much
	// of the hash was guessed correctly.
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// decode reads the parameters, salt and key back out of the hash string, so
// verification uses the settings the hash was created with rather than the
// current defaults.
func decode(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return Params{}, nil, nil, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return Params{}, nil, nil, ErrInvalidHash
	}

	if version != argon2.Version {
		return Params{}, nil, nil, fmt.Errorf("%w: version %d", ErrInvalidHash, version)
	}

	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Time, &p.Parallelism); err != nil {
		return Params{}, nil, nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, ErrInvalidHash
	}

	key, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil {
		return Params{}, nil, nil, ErrInvalidHash
	}

	p.SaltLength = uint32(len(salt))
	p.KeyLength = uint32(len(key))

	return p, salt, key, nil
}
