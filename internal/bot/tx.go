package bot

import (
	"context"

	"poraclego/internal/db"
)

func (m *Manager) withQueryTx(fn func(*db.Query) error) error {
	if m == nil || m.query == nil {
		return nil
	}
	return m.query.WithTx(context.Background(), fn)
}
