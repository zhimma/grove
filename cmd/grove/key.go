package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/spf13/cobra"
)

func newKeyGenerateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "key:generate",
		Short: "生成 jwt.secret 与 security.config_encryption_key 强随机密钥",
		Long: `打印一组新的强随机密钥，复制到 config.yaml 的对应位置。

不改写任何文件：替换已有的 jwt.secret 会让签发过的 token 全部失效，
替换 config_encryption_key 会让已加密的系统配置无法再解密。`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			jwtSecret, err := randomKey()
			if err != nil {
				return err
			}
			encryptionKey, err := randomKey()
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "jwt:\n  secret: '%s'\nsecurity:\n  config_encryption_key: 'base64:%s'\n",
				base64.RawURLEncoding.EncodeToString(jwtSecret), base64.StdEncoding.EncodeToString(encryptionKey))
			return err
		},
	}
}

// randomKey is 32 bytes: the AES-256 key size secretbox expects, and 43
// characters once encoded, past the 32 production requires of jwt.secret.
func randomKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("生成随机密钥: %w", err)
	}
	return key, nil
}
