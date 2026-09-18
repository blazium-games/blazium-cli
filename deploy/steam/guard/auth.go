package guard

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"
)

type Tokens struct {
	AccessToken  string
	RefreshToken string
	SteamID      uint64
	AccountName  string
}

type Prompter interface {
	Username() (string, error)
	Password() (string, error)
	GuardCode(kind string) (string, error)
	ConfirmRevocation(code string) error
	SMSCode() (string, error)
}

// LoginWithPassword performs IAuthenticationService RSA password login.
func LoginWithPassword(p Prompter, username, password string) (*Tokens, error) {
	c := newSteamClient()
	if username == "" {
		u, err := p.Username()
		if err != nil {
			return nil, err
		}
		username = strings.TrimSpace(u)
	}
	if password == "" {
		pw, err := p.Password()
		if err != nil {
			return nil, err
		}
		password = pw
	}
	rsaBody := encodeString(1, username)
	rm, _, err := c.call("IAuthenticationService", "GetPasswordRSAPublicKey", 1, rsaBody, "")
	if err != nil {
		return nil, err
	}
	mod := rm.str(1)
	exp := rm.str(2)
	ts := rm.u(3)
	enc, err := encryptPassword(mod, exp, password)
	if err != nil {
		return nil, err
	}
	begin := concat(
		encodeString(1, "blazium-cli"),
		encodeString(2, username),
		encodeString(3, enc),
		encodeVarintField(4, ts),
		encodeVarintField(6, 3), // mobile app
		encodeVarintField(7, 1),
		encodeString(8, "Mobile"),
	)
	bm, _, err := c.call("IAuthenticationService", "BeginAuthSessionViaCredentials", 1, begin, "")
	if err != nil {
		return nil, err
	}
	clientID := bm.u(1)
	requestID := bm.bytes(2)
	steamID := bm.u(5)
	if clientID == 0 || len(requestID) == 0 {
		return nil, fmt.Errorf("steam login: missing client_id/request_id (bad credentials?)")
	}
	needCode := false
	if nested := bm.fields[4]; len(nested) > 0 {
		needCode = true
	}
	if needCode {
		code, err := p.GuardCode("email or device")
		if err != nil {
			return nil, err
		}
		upd := concat(
			encodeVarintField(1, clientID),
			encodeFixed64(2, steamID),
			encodeString(3, strings.TrimSpace(code)),
			encodeVarintField(4, 2), // email code; Steam accepts device as 3
		)
		if _, _, err := c.call("IAuthenticationService", "UpdateAuthSessionWithSteamGuardCode", 1, upd, ""); err != nil {
			upd[len(upd)-1] = 3
			if _, _, err2 := c.call("IAuthenticationService", "UpdateAuthSessionWithSteamGuardCode", 1, concat(
				encodeVarintField(1, clientID),
				encodeFixed64(2, steamID),
				encodeString(3, strings.TrimSpace(code)),
				encodeVarintField(4, 3),
			), ""); err2 != nil {
				return nil, err
			}
		}
	}
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		poll := concat(encodeVarintField(1, clientID), encodeBytes(2, requestID))
		pm, _, err := c.call("IAuthenticationService", "PollAuthSessionStatus", 1, poll, "")
		if err != nil {
			return nil, err
		}
		if pm.str(3) != "" && pm.str(4) != "" {
			return &Tokens{
				RefreshToken: pm.str(3),
				AccessToken:  pm.str(4),
				SteamID:      steamID,
				AccountName:  firstNonEmpty(pm.str(6), username),
			}, nil
		}
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("steam login timed out waiting for tokens")
}

func encryptPassword(modHex, expHex, password string) (string, error) {
	mod := new(big.Int)
	if _, ok := mod.SetString(modHex, 16); !ok {
		return "", fmt.Errorf("bad RSA modulus")
	}
	exp := new(big.Int)
	if _, ok := exp.SetString(expHex, 16); !ok {
		return "", fmt.Errorf("bad RSA exponent")
	}
	pub := &rsa.PublicKey{N: mod, E: int(exp.Int64())}
	cipher, err := rsa.EncryptPKCS1v15(rand.Reader, pub, []byte(password))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(cipher), nil
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// Discard is used when tests need an io.Writer.
var Discard io.Writer = io.Discard
