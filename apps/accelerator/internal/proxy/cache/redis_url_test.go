package cache

import (
	"strings"
	"testing"
)

func TestRedactRedisAddr(t *testing.T) {
	in := "redis://default:s3cret@drink.example.com:18717"
	out := RedactRedisAddr(in)
	if strings.Contains(out, "s3cret") {
		t.Fatalf("password leaked: %s", out)
	}
	if !strings.Contains(out, "drink.example.com:18717") {
		t.Fatalf("host lost: %s", out)
	}
	if RedactRedisAddr("localhost:6379") != "localhost:6379" {
		t.Fatal("host:port should pass through")
	}
}

func TestNewRedisClient_ParseURL(t *testing.T) {
	// Does not dial — only constructs options via ParseURL.
	client, endpoint, err := newRedisClient(Options{
		Addr: "redis://default:AaWoaZpQxxMpeVmWKW4BEFoBpKc4kvdb@drink-ultrapolished-bushes-16514.db.redis.io:18717",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if strings.Contains(endpoint, "AaWoa") {
		t.Fatalf("endpoint should redact password: %s", endpoint)
	}
	opt := client.Options()
	if opt.Addr != "drink-ultrapolished-bushes-16514.db.redis.io:18717" {
		t.Fatalf("addr=%q", opt.Addr)
	}
	if opt.Password == "" {
		t.Fatal("expected password from URL")
	}
	if opt.Username != "default" && opt.Username != "" {
		// go-redis may put user in Username field
		t.Logf("username=%q (ok)", opt.Username)
	}
}

func TestNewRedisClient_HostPort(t *testing.T) {
	client, endpoint, err := newRedisClient(Options{Addr: "127.0.0.1:6379", Password: "x", DB: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if endpoint != "127.0.0.1:6379" {
		t.Fatalf("endpoint=%s", endpoint)
	}
	if client.Options().DB != 2 || client.Options().Password != "x" {
		t.Fatalf("%+v", client.Options())
	}
}
