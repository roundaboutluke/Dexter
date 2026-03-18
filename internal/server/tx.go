package server

import (
	"context"

	"dexter/internal/db"
)

func withAlertStateTx(s *Server, fn func(*db.Query) error) error {
	if s == nil || s.query == nil {
		return nil
	}
	return s.query.WithTx(context.Background(), fn)
}

func replaceTrackedRowsTx(s *Server, table string, scope map[string]any, updates []map[string]any, inserts []map[string]any) error {
	if len(updates)+len(inserts) == 0 {
		return nil
	}
	return withAlertStateTx(s, func(query *db.Query) error {
		if len(updates) > 0 {
			uids := make([]any, 0, len(updates))
			insertUpdates := make([]map[string]any, 0, len(updates))
			for _, row := range updates {
				if row["uid"] != nil {
					uids = append(uids, row["uid"])
				}
				clone := map[string]any{}
				for key, value := range row {
					if key == "uid" {
						continue
					}
					clone[key] = value
				}
				insertUpdates = append(insertUpdates, clone)
			}
			if len(uids) > 0 {
				if _, err := query.DeleteWhereInQuery(table, scope, uids, "uid"); err != nil {
					return err
				}
			}
			inserts = append(inserts, insertUpdates...)
		}
		if len(inserts) == 0 {
			return nil
		}
		if _, err := query.InsertQuery(table, inserts); err != nil {
			return err
		}
		return nil
	})
}
