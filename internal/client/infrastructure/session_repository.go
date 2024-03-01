// Package infrastructure содержит имплементацию репозиториев и клиентов
package infrastructure

import (
	"encoding/json"

	bolt "go.etcd.io/bbolt"

	"github.com/Nickolasll/goph-keeper/internal/client/domain"
)

// SessionRepository - Имплементация репозитория сессий
type SessionRepository struct {
	// DB - Инстанс базы данных bbolt
	DB *bolt.DB
	// Crypto - Инстанс сервиса шифрования
	Crypto domain.CryptoServiceInterface
}

// Save - Сохраняет новую сессию
func (r SessionRepository) Save(session domain.Session) error {
	buf, err := json.Marshal(session)
	if err != nil {
		return err
	}

	encrypted, err := r.Crypto.Encrypt(buf)
	if err != nil {
		return err
	}

	tx, err := r.DB.Begin(true)
	if err != nil {
		return err
	}
	defer func() {
		err = tx.Rollback()
	}()

	b, err := tx.CreateBucketIfNotExists([]byte("ActiveSession"))
	if err != nil {
		return err
	}

	err = b.Put([]byte("Session"), encrypted)
	if err != nil {
		return err
	}
	_, err = tx.CreateBucketIfNotExists([]byte(session.UserID))
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return err
}

// Get - Возвращает последнюю сессию, если она существует
func (r SessionRepository) Get() (domain.Session, error) {
	var session domain.Session
	var raw []byte

	err := r.DB.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte("ActiveSession"))
		if root == nil {
			return domain.ErrEntityNotFound
		}
		raw = root.Get([]byte("Session"))

		return nil
	})
	if err != nil {
		return session, err
	}

	decrypted, err := r.Crypto.Decrypt(raw)
	if err != nil {
		return session, err
	}

	err = json.Unmarshal(decrypted, &session)
	if err != nil {
		return session, err
	}

	return session, nil
}
