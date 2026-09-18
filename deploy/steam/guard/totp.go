package guard

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

const alphabet = "23456789BCDFGHJKMNPQRTVWXY"

// queryTimeOffsetFn is QueryTimeOffset; tests replace it to avoid the network.
var queryTimeOffsetFn = QueryTimeOffset

var steamOffsetCache struct {
	mu   sync.Mutex
	unix int64
	off  int64
}

func steamClockOffset() int64 {
	steamOffsetCache.mu.Lock()
	defer steamOffsetCache.mu.Unlock()
	now := time.Now().Unix()
	if steamOffsetCache.unix != 0 && now-steamOffsetCache.unix < 30 {
		return steamOffsetCache.off
	}
	off, err := queryTimeOffsetFn()
	if err != nil {
		return 0
	}
	steamOffsetCache.unix = now
	steamOffsetCache.off = off
	return off
}

// GenerateAuthCode is a Steam-style 5-character TOTP code (not RFC 6238 digits).
// timeOffset is added on top of Steam QueryTime vs local clock (best-effort; 0 if QueryTime fails).
func GenerateAuthCode(secret string, timeOffset int64) (string, error) {
	key, err := decodeSecret(secret)
	if err != nil {
		return "", err
	}
	unix := time.Now().Unix() + steamClockOffset() + timeOffset
	return codeAt(key, unix), nil
}

// GenerateAuthCodeAt uses a frozen unix timestamp.
func GenerateAuthCodeAt(secret string, unix int64) (string, error) {
	key, err := decodeSecret(secret)
	if err != nil {
		return "", err
	}
	return codeAt(key, unix), nil
}

func codeAt(secret []byte, unix int64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(unix/30))
	mac := hmac.New(sha1.New, secret)
	mac.Write(buf[:])
	h := mac.Sum(nil)
	start := int(h[19] & 0x0F)
	full := binary.BigEndian.Uint32(h[start:start+4]) & 0x7FFFFFFF
	var code [5]byte
	for i := 0; i < 5; i++ {
		code[i] = alphabet[full%uint32(len(alphabet))]
		full /= uint32(len(alphabet))
	}
	return string(code[:])
}

func decodeSecret(secret string) ([]byte, error) {
	s := strings.TrimSpace(secret)
	if s == "" {
		return nil, fmt.Errorf("empty shared_secret")
	}
	if matched, _ := hexish(s); matched {
		b, err := hex.DecodeString(s)
		if err == nil && len(b) > 0 {
			return b, nil
		}
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("shared_secret: %w", err)
	}
	if len(b) == 0 {
		return nil, fmt.Errorf("shared_secret decoded empty")
	}
	return b, nil
}

func hexish(s string) (bool, error) {
	if len(s) != 40 {
		return false, nil
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false, nil
		}
	}
	return true, nil
}

// GenerateConfirmationKey HMAC-SHA1(identity_secret, time_be64 || tag) as base64.
func GenerateConfirmationKey(identitySecret string, unix int64, tag string) (string, error) {
	key, err := decodeSecret(identitySecret)
	if err != nil {
		return "", err
	}
	n := 8
	if tag != "" {
		if len(tag) > 32 {
			n += 32
		} else {
			n += len(tag)
		}
	}
	buf := make([]byte, n)
	binary.BigEndian.PutUint64(buf[:8], uint64(unix))
	if tag != "" {
		copy(buf[8:], tag)
	}
	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

// DeviceID is android: + SHA1(steamid) as a UUID-shaped hex string.
func DeviceID(steamID string) string {
	sum := sha1.Sum([]byte(steamID))
	h := hex.EncodeToString(sum[:])
	if len(h) < 32 {
		return "android:" + h
	}
	return "android:" + h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// WindowCode returns a login code that will not straddle a 30s boundary.
// Codes use Steam QueryTime vs local clock (best-effort). If offset 0 and +5s
// differ, wait until the later window (same as steam_deploy.sh).
func WindowCode(secret string) (string, error) {
	a, err := GenerateAuthCode(secret, 0)
	if err != nil {
		return "", err
	}
	b, err := GenerateAuthCode(secret, 5)
	if err != nil {
		return "", err
	}
	if a != b {
		time.Sleep(6 * time.Second)
		return b, nil
	}
	return a, nil
}
