package accountcmds

import (
	"log"

	"github.com/james-lawrence/eg/cmd/cmdopts"
)

type Register struct {
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
}

func (t Register) Run(gctx *cmdopts.Global) (err error) {
	log.Println("signing up")
	// var (
	// 	signer ssh.Signer
	// 	sig    *ssh.Signature
	// )

	// if signer, err = sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath); err != nil {
	// 	return err
	// }

	// password := uuid.Must(uuid.NewV4())

	// ctransport := &http.Transport{
	// 	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	// }
	// chttp := &http.Client{Transport: ctransport, Timeout: 10 * time.Second}

	// ctx := context.WithValue(context.Background(), oauth2.HTTPClient, chttp)
	// cfg := oauth2.Config{
	// 	ClientID:     ssh.FingerprintSHA256(signer.PublicKey()),
	// 	ClientSecret: password.String(),
	// 	Endpoint: oauth2.Endpoint{
	// 		AuthURL:   fmt.Sprintf("%s/oauth2/ssh/auth", envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost)),
	// 		TokenURL:  fmt.Sprintf("%s/oauth2/ssh/token", envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost)),
	// 		AuthStyle: oauth2.AuthStyleInHeader,
	// 	},
	// }

	// if sig, err = signer.Sign(rand.Reader, uuid.FromStringOrNil(cfg.ClientSecret).Bytes()); err != nil {
	// 	return err
	// }

	// token, err := cfg.PasswordCredentialsToken(ctx, cfg.ClientID, base64.RawURLEncoding.EncodeToString(ssh.Marshal(sig)))
	// if err != nil {
	// 	return err
	// }

	// httpc := cfg.Client(ctx, token)
	// resp, err := httpc.Post(fmt.Sprintf("%s/authn/ssh", envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost)), "application/json", nil)
	// if err != nil {
	// 	return err
	// }

	// if decoded, err := httputil.DumpResponse(resp, true); err == nil {
	// 	log.Println("DERP", string(decoded))
	// }

	return nil
}
