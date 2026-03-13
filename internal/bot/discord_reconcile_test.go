package bot

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/bwmarrin/discordgo"

	"poraclego/internal/config"
)

func TestSyncDiscordChannelsSkipsTransientFetchErrors(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{
		"humans": {{
			"id":            "chan-1",
			"type":          "discord:channel",
			"name":          "raids",
			"admin_disable": 0,
		}},
	}, 0)
	env.discord.channelFetcher = func(*discordgo.Session, string) (*discordgo.Channel, error) {
		return nil, newDiscordRESTError(http.StatusServiceUnavailable, 0, "service unavailable")
	}

	changed := env.discord.syncDiscordChannels(&discordgo.Session{}, false, false, true)
	if changed {
		t.Fatalf("changed=true, want false")
	}

	row, err := env.query.SelectOneQuery("humans", map[string]any{"id": "chan-1"})
	if err != nil {
		t.Fatalf("select human: %v", err)
	}
	if got := toInt(row["admin_disable"], 0); got != 0 {
		t.Fatalf("admin_disable=%d, want 0", got)
	}
	if row["disabled_date"] != nil {
		t.Fatalf("disabled_date=%v, want nil", row["disabled_date"])
	}
}

func TestSyncDiscordChannelsDisablesUnknownChannel(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{
		"humans": {{
			"id":            "chan-1",
			"type":          "discord:channel",
			"name":          "raids",
			"admin_disable": 0,
		}},
	}, 0)
	env.discord.channelFetcher = func(*discordgo.Session, string) (*discordgo.Channel, error) {
		return nil, newDiscordRESTError(http.StatusNotFound, discordUnknownChannelCode, "Unknown Channel")
	}

	changed := env.discord.syncDiscordChannels(&discordgo.Session{}, false, false, true)
	if !changed {
		t.Fatalf("changed=false, want true")
	}

	row, err := env.query.SelectOneQuery("humans", map[string]any{"id": "chan-1"})
	if err != nil {
		t.Fatalf("select human: %v", err)
	}
	if got := toInt(row["admin_disable"], 0); got != 1 {
		t.Fatalf("admin_disable=%d, want 1", got)
	}
	if row["disabled_date"] == nil {
		t.Fatalf("disabled_date=nil, want timestamp")
	}
}

func TestSyncDiscordRoleIncompleteLoadSkipsDestructivePass(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{
		"humans": {{
			"id":            "user-1",
			"type":          "discord:user",
			"name":          "User One",
			"admin_disable": 0,
		}},
	}, 0)
	env.discord.manager.cfg = config.New(map[string]any{
		"discord": map[string]any{
			"guilds":   []string{"g1", "g2"},
			"userRole": []string{"role-a"},
		},
		"general": map[string]any{
			"roleCheckMode": "disable-user",
		},
		"areaSecurity": map[string]any{
			"enabled": false,
		},
	})
	env.discord.guildMembersLoader = func(_ *discordgo.Session, guildID string) ([]*discordgo.Member, error) {
		if guildID == "g1" {
			return []*discordgo.Member{}, nil
		}
		return nil, newDiscordRESTError(http.StatusBadGateway, 0, "gateway timeout")
	}

	changed := env.discord.syncDiscordRole(&discordgo.Session{}, false, false, true)
	if changed {
		t.Fatalf("changed=true, want false")
	}

	row, err := env.query.SelectOneQuery("humans", map[string]any{"id": "user-1"})
	if err != nil {
		t.Fatalf("select human: %v", err)
	}
	if got := toInt(row["admin_disable"], 0); got != 0 {
		t.Fatalf("admin_disable=%d, want 0", got)
	}
	if row["disabled_date"] != nil {
		t.Fatalf("disabled_date=%v, want nil", row["disabled_date"])
	}
}

func TestSyncDiscordRoleCompleteLoadStillDisablesInvalidUsers(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{
		"humans": {{
			"id":            "user-1",
			"type":          "discord:user",
			"name":          "User One",
			"admin_disable": 0,
		}},
	}, 0)
	env.discord.manager.cfg = config.New(map[string]any{
		"discord": map[string]any{
			"guilds":   []string{"g1"},
			"userRole": []string{"role-a"},
		},
		"general": map[string]any{
			"roleCheckMode": "disable-user",
		},
		"areaSecurity": map[string]any{
			"enabled": false,
		},
	})
	env.discord.guildMembersLoader = func(*discordgo.Session, string) ([]*discordgo.Member, error) {
		return []*discordgo.Member{}, nil
	}

	changed := env.discord.syncDiscordRole(&discordgo.Session{}, false, false, true)
	if !changed {
		t.Fatalf("changed=false, want true")
	}

	row, err := env.query.SelectOneQuery("humans", map[string]any{"id": "user-1"})
	if err != nil {
		t.Fatalf("select human: %v", err)
	}
	if got := toInt(row["admin_disable"], 0); got != 1 {
		t.Fatalf("admin_disable=%d, want 1", got)
	}
	if row["disabled_date"] == nil {
		t.Fatalf("disabled_date=nil, want timestamp")
	}
}

func TestReconcileUserSkipsTransientFetchErrors(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{
		"humans": {{
			"id":            "user-1",
			"type":          "discord:user",
			"name":          "User One",
			"admin_disable": 0,
		}},
	}, 0)
	env.discord.manager.cfg = config.New(map[string]any{
		"discord": map[string]any{
			"guilds":   []string{"g1"},
			"userRole": []string{"role-a"},
		},
		"general": map[string]any{
			"roleCheckMode": "disable-user",
		},
		"areaSecurity": map[string]any{
			"enabled": false,
		},
		"reconciliation": map[string]any{
			"discord": map[string]any{
				"removeInvalidUsers": true,
			},
		},
	})
	env.discord.guildMemberFetcher = func(*discordgo.Session, string, string) (*discordgo.Member, error) {
		return nil, newDiscordRESTError(http.StatusServiceUnavailable, 0, "service unavailable")
	}

	env.discord.reconcileUser(&discordgo.Session{}, "user-1")

	env.assertNoRefresh(t)

	row, err := env.query.SelectOneQuery("humans", map[string]any{"id": "user-1"})
	if err != nil {
		t.Fatalf("select human: %v", err)
	}
	if got := toInt(row["admin_disable"], 0); got != 0 {
		t.Fatalf("admin_disable=%d, want 0", got)
	}
}

func TestReconcileUserUnknownMemberStillDisablesWhenConfigured(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{
		"humans": {{
			"id":            "user-1",
			"type":          "discord:user",
			"name":          "User One",
			"admin_disable": 0,
		}},
	}, 0)
	env.discord.manager.cfg = config.New(map[string]any{
		"discord": map[string]any{
			"guilds":   []string{"g1"},
			"userRole": []string{"role-a"},
		},
		"general": map[string]any{
			"roleCheckMode": "disable-user",
		},
		"areaSecurity": map[string]any{
			"enabled": false,
		},
		"reconciliation": map[string]any{
			"discord": map[string]any{
				"removeInvalidUsers": true,
			},
		},
	})
	env.discord.guildMemberFetcher = func(*discordgo.Session, string, string) (*discordgo.Member, error) {
		return nil, newDiscordRESTError(http.StatusNotFound, 10007, "Unknown Member")
	}

	env.discord.reconcileUser(&discordgo.Session{}, "user-1")

	env.waitForRefresh(t, 1)

	row, err := env.query.SelectOneQuery("humans", map[string]any{"id": "user-1"})
	if err != nil {
		t.Fatalf("select human: %v", err)
	}
	if got := toInt(row["admin_disable"], 0); got != 1 {
		t.Fatalf("admin_disable=%d, want 1", got)
	}
}

func newDiscordRESTError(status, code int, message string) error {
	return &discordgo.RESTError{
		Response: &http.Response{
			StatusCode: status,
			Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		},
		Message: &discordgo.APIErrorMessage{
			Code:    code,
			Message: message,
		},
	}
}
