package command

import (
	"context"

	"poraclego/internal/db"
)

func withAlertStateTx(ctx *Context, fn func(*db.Query) error) error {
	if ctx == nil || ctx.Query == nil {
		return nil
	}
	return ctx.Query.WithTx(context.Background(), fn)
}

func commitAlertStateTx(ctx *Context, fn func(*db.Query) error) error {
	if err := withAlertStateTx(ctx, fn); err != nil {
		return err
	}
	if ctx != nil {
		ctx.MarkAlertStateDirty()
	}
	return nil
}

func replaceTrackedRowsTx(ctx *Context, table string, scope map[string]any, updates []map[string]any, inserts []map[string]any) error {
	if len(updates)+len(inserts) == 0 {
		return nil
	}
	return commitAlertStateTx(ctx, func(query *db.Query) error {
		if len(updates) > 0 {
			if _, err := query.DeleteWhereInQuery(table, scope, extractUids(updates), "uid"); err != nil {
				return err
			}
		}
		if _, err := query.InsertQuery(table, append(inserts, updates...)); err != nil {
			return err
		}
		return nil
	})
}
