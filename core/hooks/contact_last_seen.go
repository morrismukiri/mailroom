package hooks

import (
	"context"

	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/mailroom/core/models"

	"github.com/edganiukov/fcm"
	"github.com/gomodule/redigo/redis"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

// ContactLastSeenHook is our hook for contact changes that require an update to last_seen_on
var ContactLastSeenHook models.EventCommitHook = &contactLastSeenHook{}

type contactLastSeenHook struct{}

// Apply squashes and updates modified_on on all the contacts passed in
func (h *contactLastSeenHook) Apply(ctx context.Context, tx *sqlx.Tx, rp *redis.Pool, fc *fcm.Client, oa *models.OrgAssets, scenes map[*models.Scene][]interface{}) error {

	for scene, evts := range scenes {
		lastEvent := evts[len(evts)-1].(flows.Event)
		lastSeenOn := lastEvent.CreatedOn()

		err := models.UpdateContactLastSeenOn(ctx, tx, scene.ContactID(), lastSeenOn)
		if err != nil {
			return errors.Wrapf(err, "error updating last_seen_on on contacts")
		}
	}

	return nil
}
