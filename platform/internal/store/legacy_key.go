package store

import "fmt"

// legacyMasterVaultService 是 1.x 主密钥在 OS Keychain 中的条目名。
const legacyMasterVaultService = "NiuMa/master-vault"

// AdoptLegacyMasterKey 在 ensis-ops/master-vault 还不存在时，复制 1.x 的主密钥。
//
// 密文在 SQLite 里，解密依赖这一条主密钥。旧条目保留，方便回退。
// 新条目已存在时不覆盖。1.x 没有主密钥时 adopted 为 false。
func AdoptLegacyMasterKey(ring SecretStore) (bool, error) {
	if ring == nil {
		return false, fmt.Errorf("store: adopt legacy master key: nil keyring")
	}
	_, ok, err := ring.GetSecret(masterVaultService, masterVaultAccount)
	if err != nil {
		return false, fmt.Errorf("store: adopt legacy master key: %w", err)
	}
	if ok {
		return false, nil
	}
	secret, found, err := ring.GetSecret(legacyMasterVaultService, masterVaultAccount)
	if err != nil {
		return false, fmt.Errorf("store: adopt legacy master key: %w", err)
	}
	if !found || secret == "" {
		return false, nil
	}
	if err := ring.SetSecret(masterVaultService, masterVaultAccount, secret); err != nil {
		return false, fmt.Errorf("store: adopt legacy master key: %w", err)
	}
	return true, nil
}
