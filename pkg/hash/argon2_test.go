package hash_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/leftmy/planetarium-server/pkg/hash"
)

func TestVerify(t *testing.T) {
	const password = "correct horse battery staple"

	encoded, err := hash.Password(password)
	if err != nil {
		t.Fatalf("password: %v", err)
	}

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"correct password", password, true},
		{"wrong password", "Correct horse battery staple", false},
		{"empty password", "", false},
		{"password with a trailing space", password + " ", false},
		{"the hash itself", encoded, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hash.Verify(tt.input, encoded)
			if err != nil {
				t.Fatalf("verify: %v", err)
			}

			if got != tt.want {
				t.Errorf("verify = %v, want %v", got, tt.want)
			}
		})
	}
}

// The salt must be random, so the same password hashed twice must not produce
// the same string. Equal hashes would let anyone spot shared passwords in a
// database dump.
func TestPasswordSaltsEachHash(t *testing.T) {
	const password = "correct horse battery staple"

	first, err := hash.Password(password)
	if err != nil {
		t.Fatalf("first: %v", err)
	}

	second, err := hash.Password(password)
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if first == second {
		t.Fatal("hashing the same password twice produced identical strings")
	}

	// Both must still verify.
	for i, encoded := range []string{first, second} {
		ok, err := hash.Verify(password, encoded)
		if err != nil {
			t.Fatalf("verify %d: %v", i, err)
		}

		if !ok {
			t.Errorf("hash %d does not verify its own password", i)
		}
	}
}

// The point of storing the parameters inside the hash: after the defaults are
// raised, passwords hashed with the old settings must still let their owners in.
// Getting this wrong locks out every existing user the day the cost is changed.
func TestVerifyUsesTheParametersInTheHash(t *testing.T) {
	const password = "correct horse battery staple"

	original := hash.DefaultParams

	t.Cleanup(func() { hash.DefaultParams = original })

	// A deliberately cheap setting, standing in for "the parameters we used to use".
	hash.DefaultParams = hash.Params{
		Memory:      8 * 1024,
		Time:        1,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}

	old, err := hash.Password(password)
	if err != nil {
		t.Fatalf("password with old params: %v", err)
	}

	if !strings.Contains(old, "m=8192,t=1,p=1") {
		t.Fatalf("hash does not carry its parameters: %q", old)
	}

	// Now the cost goes up, as it would in a future commit.
	hash.DefaultParams = original

	ok, err := hash.Verify(password, old)
	if err != nil {
		t.Fatalf("verify old hash: %v", err)
	}

	if !ok {
		t.Error("a password hashed with the previous parameters no longer verifies")
	}
}

// A stored string that is not a readable hash must be an error, not a silent
// "wrong password": the two need different handling, and treating corruption as
// a failed login hides it.
func TestVerifyRejectsMalformedHashes(t *testing.T) {
	valid, err := hash.Password("whatever")
	if err != nil {
		t.Fatalf("password: %v", err)
	}

	tests := []struct {
		name    string
		encoded string
	}{
		{"empty", ""},
		{"not a hash", "plaintext"},
		{"wrong algorithm", strings.Replace(valid, "argon2id", "argon2i", 1)},
		{"missing fields", "$argon2id$v=19$m=65536,t=3,p=2"},
		{"unknown version", strings.Replace(valid, "v=19", "v=18", 1)},
		{"salt is not base64", "$argon2id$v=19$m=65536,t=3,p=2$!!!!$" + strings.Split(valid, "$")[5]},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := hash.Verify("whatever", tt.encoded)
			if ok {
				t.Error("a malformed hash verified successfully")
			}

			if !errors.Is(err, hash.ErrInvalidHash) {
				t.Errorf("err = %v, want %v", err, hash.ErrInvalidHash)
			}
		})
	}
}

// Login verifies against DummyHash when no account matches, so it has to be a
// hash the package can actually read. A malformed constant would turn every
// unknown-email login into a 500.
func TestDummyHashIsUsable(t *testing.T) {
	ok, err := hash.Verify("any password at all", hash.DummyHash)
	if err != nil {
		t.Fatalf("verify against DummyHash: %v", err)
	}

	if ok {
		t.Error("DummyHash verified a password, which defeats its purpose")
	}
}
