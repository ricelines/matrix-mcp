package tuwunel

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"maunium.net/go/mautrix"
)

const image = "ghcr.io/matrix-construct/tuwunel:v1.5.0"

type Options struct {
	RegistrationToken string
}

type Instance struct {
	containerName     string
	baseDir           string
	HomeserverURL     string
	RegistrationToken string
}

type registrationTokenAuthData struct {
	mautrix.BaseAuthData
	Token string `json:"token"`
}

func Start(t testing.TB, opts Options) *Instance {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker is required for integration tests")
	}

	baseDir := t.TempDir()
	serverDir := filepath.Join(baseDir, "tuwunel")
	if err := os.MkdirAll(filepath.Join(serverDir, "database"), 0o755); err != nil {
		t.Fatalf("mkdir database: %v", err)
	}
	if err := os.WriteFile(filepath.Join(serverDir, "tuwunel.toml"), []byte(config("/data/database", opts.RegistrationToken)), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	hostPort := reservePort(t)
	containerName := fmt.Sprintf("matrix-mcp-go-%d", time.Now().UnixNano())
	inst := &Instance{
		containerName:     containerName,
		baseDir:           baseDir,
		HomeserverURL:     fmt.Sprintf("http://127.0.0.1:%d", hostPort),
		RegistrationToken: opts.RegistrationToken,
	}

	cmd := exec.Command("docker", "run", "-d", "--rm", "--name", containerName,
		"-e", "TUWUNEL_CONFIG=/data/tuwunel.toml",
		"-v", fmt.Sprintf("%s:/data", serverDir),
		"-p", fmt.Sprintf("%d:8008", hostPort),
		image,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("start docker container: %v\n%s", err, output)
	}

	t.Cleanup(func() {
		_ = exec.Command("docker", "stop", containerName).Run()
	})
	return inst
}

func (i *Instance) WaitUntilReady(ctx context.Context) error {
	deadline := time.Now().Add(60 * time.Second)
	client := &http.Client{Timeout: 2 * time.Second}
	url := i.HomeserverURL + "/_matrix/client/versions"
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode/100 == 2 {
				return nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("tuwunel did not become ready at %s", url)
}

func (i *Instance) RegisterUser(ctx context.Context, username, password string) error {
	client, err := mautrix.NewClient(i.HomeserverURL, "", "")
	if err != nil {
		return err
	}
	registerReq := &mautrix.ReqRegister{Username: username, Password: password}
	if i.RegistrationToken == "" {
		_, err = client.RegisterDummy(ctx, registerReq)
		return err
	}
	resp, uia, err := client.Register(ctx, registerReq)
	if err != nil && uia == nil {
		return err
	}
	if resp != nil {
		return nil
	}
	if uia == nil || !uia.HasSingleStageFlow(mautrix.AuthType("m.login.registration_token")) {
		return fmt.Errorf("homeserver did not advertise registration-token auth")
	}
	registerReq.Auth = registrationTokenAuthData{
		BaseAuthData: mautrix.BaseAuthData{Type: mautrix.AuthType("m.login.registration_token"), Session: uia.Session},
		Token:        i.RegistrationToken,
	}
	_, _, err = client.Register(ctx, registerReq)
	return err
}

func (i *Instance) LoginClient(ctx context.Context, username, password string) (*mautrix.Client, error) {
	client, err := mautrix.NewClient(i.HomeserverURL, "", "")
	if err != nil {
		return nil, err
	}
	_, err = client.Login(ctx, &mautrix.ReqLogin{
		Type: mautrix.AuthTypePassword,
		Identifier: mautrix.UserIdentifier{
			Type: mautrix.IdentifierTypeUser,
			User: username,
		},
		Password:         password,
		StoreCredentials: true,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

func reservePort(t testing.TB) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func config(databasePath string, registrationToken string) string {
	body := fmt.Sprintf("[global]\nserver_name = \"localhost\"\ndatabase_path = %q\naddress = \"0.0.0.0\"\nport = 8008\nallow_registration = true\nallow_legacy_media = true\nallow_public_room_directory_without_auth = true\ncreate_admin_room = false\nerror_on_unknown_config_opts = true\nquery_trusted_key_servers_first = false\nquery_trusted_key_servers_first_on_join = false\nyes_i_am_very_very_sure_i_want_an_open_registration_server_prone_to_abuse = true\ntrusted_servers = []\n", databasePath)
	if registrationToken != "" {
		body += fmt.Sprintf("registration_token = %q\n", registrationToken)
	}
	return body
}
