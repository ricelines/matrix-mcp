package matrix

import (
	"context"
	"fmt"
	"log"

	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/crypto/backup"
	"maunium.net/go/mautrix/event"
)

// bootstrapFromRecoveryKey cross-signs this device with the account's
// cross-signing keys from SSSS and imports the server-side megolm key backup.
//
// Cross-signing is what makes other clients (e.g. mautrix bridges with
// verification_levels.share = cross-signed-tofu) share room keys with this
// device. The key backup import makes history from before this device existed
// decryptable. Meant to run once: remove the recovery key from the
// environment after a successful start.
func bootstrapFromRecoveryKey(ctx context.Context, mach *crypto.OlmMachine, recoveryKey string) error {
	if err := mach.VerifyWithRecoveryKey(ctx, recoveryKey); err != nil {
		return fmt.Errorf("cross-sign device with recovery key: %w", err)
	}
	log.Printf("recovery key: device %s cross-signed", mach.Client.DeviceID)

	keyID, keyData, err := mach.SSSS.GetDefaultKeyData(ctx)
	if err != nil {
		return fmt.Errorf("get default SSSS key: %w", err)
	}
	key, err := keyData.VerifyRecoveryKey(keyID, recoveryKey)
	if err != nil {
		return fmt.Errorf("verify recovery key: %w", err)
	}
	rawBackupKey, err := mach.SSSS.GetDecryptedAccountData(ctx, event.AccountDataMegolmBackupKey, key)
	if err != nil {
		// Cross-signing alone is enough for new messages; a missing backup only limits history.
		log.Printf("recovery key: no megolm backup key in SSSS, skipping history import: %v", err)
		return nil
	}
	backupKey, err := backup.MegolmBackupKeyFromBytes(rawBackupKey)
	if err != nil {
		return fmt.Errorf("parse megolm backup key: %w", err)
	}
	version, err := mach.DownloadAndStoreLatestKeyBackup(ctx, backupKey)
	if err != nil {
		log.Printf("recovery key: key backup import failed, history may be undecryptable: %v", err)
		return nil
	}
	log.Printf("recovery key: imported key backup version %q; remove MATRIX_RECOVERY_KEY now", version)
	return nil
}
