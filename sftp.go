package sftps

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SecureFtp struct {
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	params     *sftpParameters
	state      int
}

func newSftp(p *sftpParameters) (sftp *SecureFtp) {
	sftp = new(SecureFtp)
	sftp.params = p
	return
}

func (this *SecureFtp) connect() (err error) {
	var ip []net.IP

	config := &ssh.ClientConfig{
		User: this.params.user,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	if this.params.useKey {
		var pemBytes []byte
		var signer ssh.Signer
		if strings.HasPrefix(this.params.privateKey, FILEPROTOCOL) {
			privateKey := strings.TrimPrefix(this.params.privateKey, FILEPROTOCOL)
			pemBytes, err = ioutil.ReadFile(privateKey)
			if err != nil {
				return fmt.Errorf(`Private Key File "%v": %v`, privateKey, err)
			}
		} else {
			pemBytes = []byte(this.params.privateKey)
		}
		if this.params.usePassphrase {
			passphraseBytes := []byte(this.params.passphrase)
			signer, err = ssh.ParsePrivateKeyWithPassphrase(pemBytes, passphraseBytes)
		} else {
			signer, err = ssh.ParsePrivateKey(pemBytes)
		}
		if err != nil {
			return
		}
		config.Auth = append(config.Auth, ssh.PublicKeys(signer))
	}

	if len(this.params.pass) > 0 {
		config.Auth = append(config.Auth, ssh.Password(this.params.pass))
	}

	config.SetDefaults()
	if ip, err = net.LookupIP(this.params.host); err != nil {
		return
	}
	addr := fmt.Sprintf("%s:%d", ip[0], this.params.port)

	if this.sshClient, err = ssh.Dial("tcp", addr, config); err != nil {
		return
	}
	if this.sftpClient, err = sftp.NewClient(this.sshClient); err != nil {
		if e := this.sshClient.Close(); e != nil {
			return e
		}
	}
	return
}

func (this *SecureFtp) list(p string) (list string, err error) {
	var session *ssh.Session
	if session, err = this.sshClient.NewSession(); err != nil {
		if e := this.quit(); e != nil {
			return ``, e
		}
	}
	defer session.Close()

	cmd := fmt.Sprintf("ls -al %s", p)
	var bytes []byte
	if bytes, err = session.Output(cmd); err != nil {
		if e := this.quit(); e != nil {
			return ``, e
		}
	}
	list = string(bytes)
	return
}

func (this *SecureFtp) download(local interface{}, remote string) (len int64, err error) {
	var w io.WriteCloser
	var r io.ReadCloser
	var ok bool
	if w, ok = local.(io.WriteCloser); !ok {
		if w, err = os.Create(local.(string)); err != nil {
			if e := this.quit(); e != nil {
				return 0, e
			}
		}
	}
	defer w.Close()
	if r, err = this.sftpClient.Open(remote); err != nil {
		if e := this.quit(); e != nil {
			return 0, e
		}
	}
	defer r.Close()
	if len, err = io.Copy(w, r); err != nil {
		if e := this.quit(); e != nil {
			return 0, e
		}
	}
	return
}

func (this *SecureFtp) upload(local interface{}, remote string) (len int64, err error) {
	var r io.ReadCloser
	var ok bool
	if r, ok = local.(io.ReadCloser); !ok {
		if r, err = os.Open(local.(string)); err != nil {
			if e := this.quit(); e != nil {
				return 0, e
			}
		}
	}
	defer r.Close()
	var w io.WriteCloser
	if w, err = this.sftpClient.Create(remote); err != nil {
		if e := this.quit(); e != nil {
			return 0, e
		}
	}
	defer w.Close()
	if len, err = io.Copy(w, r); err != nil {
		if e := this.quit(); e != nil {
			return 0, e
		}
	}

	return
}

func (this *SecureFtp) mkdir(p string) (err error) {
	if err = this.sftpClient.Mkdir(p); err != nil {
		if e := this.quit(); e != nil {
			panic(e)
		}
	}
	return
}

func (this *SecureFtp) remove(p string) (err error) {
	if err = this.sftpClient.Remove(p); err != nil {
		if e := this.quit(); e != nil {
			panic(e)
		}
	}
	return
}

func (this *SecureFtp) rename(old, new string) (err error) {
	if err = this.sftpClient.Rename(old, new); err != nil {
		if e := this.quit(); e != nil {
			panic(e)
		}
	}
	return
}

func (this *SecureFtp) symlink(dest, src string) (err error) {
	if err = this.sftpClient.Symlink(src, dest); err != nil {
		if e := this.quit(); e != nil {
			panic(e)
		}
	}
	return
}

func (this *SecureFtp) quit() (err error) {
	if err = this.sftpClient.Close(); err != nil {
		return
	}
	if err = this.sshClient.Close(); err != nil {
		return
	}
	return
}
