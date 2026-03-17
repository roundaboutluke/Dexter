package bot

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"poraclego/internal/geofence"
	"poraclego/internal/logging"
	"poraclego/internal/tileserver"
)

const (
	slashMapFailureBackoff = 30 * time.Second
	slashAreaMapSuccessTTL = 30 * time.Minute
	slashLocationMapTTL    = 10 * time.Minute
	slashRenderStateTTL    = 10 * time.Minute
)

type slashMapRequest struct {
	Key       string
	Kind      string
	Area      string
	Latitude  float64
	Longitude float64
}

type slashMapCacheEntry struct {
	URL        string
	ExpiresAt  time.Time
	RetryAfter time.Time
}

type slashMapJob struct {
	StartedAt time.Time
}

type slashRenderState struct {
	Revision  int64
	MapKey    string
	Target    slashRenderTarget
	BaseEmbed *discordgo.MessageEmbed
	ExpiresAt time.Time
}

type slashRenderSnapshot struct {
	Target   slashRenderTarget
	Revision int64
}

type slashRenderTargetKind string

const (
	slashRenderTargetChannel  slashRenderTargetKind = "channel"
	slashRenderTargetOriginal slashRenderTargetKind = "original"
)

type slashRenderTarget struct {
	Kind      slashRenderTargetKind
	ChannelID string
	MessageID string
	AppID     string
	Token     string
}

type slashRenderPatch struct {
	Target      slashRenderTarget
	ChannelEdit *discordgo.MessageEdit
	WebhookEdit *discordgo.WebhookEdit
}

type slashMapGenerator interface {
	Generate(req *slashMapRequest) (string, error)
}

type slashTileserverGenerator struct {
	discord *Discord
}

func (g *slashTileserverGenerator) Generate(req *slashMapRequest) (string, error) {
	if g == nil || g.discord == nil || req == nil {
		return "", fmt.Errorf("slash map generator missing context")
	}
	if g.discord.manager == nil || g.discord.manager.cfg == nil {
		return "", fmt.Errorf("slash map generator missing config")
	}
	client := tileserver.NewClient(g.discord.manager.cfg)
	switch req.Kind {
	case "area":
		if g.discord.manager.fences == nil {
			return "", fmt.Errorf("slash map generator missing fences")
		}
		return tileserver.GenerateGeofenceTile(g.discord.manager.fences.Fences, client, g.discord.manager.cfg, req.Area)
	case "location":
		return tileserver.GenerateConfiguredLocationTile(client, g.discord.manager.cfg, req.Latitude, req.Longitude)
	default:
		return "", fmt.Errorf("unsupported slash map kind %q", req.Kind)
	}
}

func (d *Discord) ensureSlashMapState() {
	if d == nil {
		return
	}
	d.mapMu.Lock()
	if d.mapCache == nil {
		d.mapCache = map[string]slashMapCacheEntry{}
	}
	if d.mapJobs == nil {
		d.mapJobs = map[string]*slashMapJob{}
	}
	if d.mapGenerator == nil {
		d.mapGenerator = &slashTileserverGenerator{discord: d}
	}
	d.mapMu.Unlock()

	d.renderMu.Lock()
	if d.renderState == nil {
		d.renderState = map[string]*slashRenderState{}
	}
	d.renderMu.Unlock()
}

func (d *Discord) slashMapsEnabled() bool {
	if d == nil || d.manager == nil || d.manager.cfg == nil {
		return false
	}
	provider, _ := d.manager.cfg.GetString("geocoding.staticProvider")
	if !strings.EqualFold(provider, "tileservercache") {
		return false
	}
	baseURL, _ := d.manager.cfg.GetString("geocoding.staticProviderURL")
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(baseURL)), "http")
}

func (d *Discord) locationMapsEnabled() bool {
	if !d.slashMapsEnabled() || d == nil || d.manager == nil || d.manager.cfg == nil {
		return false
	}
	opts := tileserver.GetOptions(d.manager.cfg, "location")
	return !strings.EqualFold(opts.Type, "none")
}

func normalizeFenceKeyValue(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "_", " "))
}

func selectFenceByName(fences []geofence.Fence, area string) *geofence.Fence {
	needle := normalizeFenceKeyValue(area)
	if needle == "" {
		return nil
	}
	for idx := range fences {
		name := normalizeFenceKeyValue(fences[idx].Name)
		if name == needle {
			return &fences[idx]
		}
	}
	return nil
}

func fencePathHash(fence *geofence.Fence) string {
	if fence == nil {
		return ""
	}
	if len(fence.Path) == 0 && len(fence.MultiPath) == 0 {
		return ""
	}
	raw, err := json.Marshal(struct {
		Path      [][]float64   `json:"path,omitempty"`
		MultiPath [][][]float64 `json:"multipath,omitempty"`
	}{
		Path:      fence.Path,
		MultiPath: fence.MultiPath,
	})
	if err != nil {
		return ""
	}
	sum := md5.Sum(raw)
	return hex.EncodeToString(sum[:])
}

func (d *Discord) areaMapRequest(area string) *slashMapRequest {
	if d == nil || d.manager == nil || d.manager.fences == nil {
		return nil
	}
	fence := selectFenceByName(d.manager.fences.Fences, area)
	if fence == nil {
		return nil
	}
	hash := fencePathHash(fence)
	if hash == "" {
		return nil
	}
	return &slashMapRequest{
		Key:  "area:" + hash,
		Kind: "area",
		Area: fence.Name,
	}
}

func (d *Discord) locationMapRequest(lat, lon float64) *slashMapRequest {
	if lat == 0 && lon == 0 {
		return nil
	}
	return &slashMapRequest{
		Key:       "location:" + formatFloat(lat) + "," + formatFloat(lon),
		Kind:      "location",
		Latitude:  lat,
		Longitude: lon,
	}
}

func (d *Discord) profileMapRequest(lat, lon float64, areas []string) *slashMapRequest {
	if lat != 0 || lon != 0 {
		if d.locationMapsEnabled() {
			return d.locationMapRequest(lat, lon)
		}
		return nil
	}
	if len(areas) > 0 {
		return d.areaMapRequest(areas[0])
	}
	return nil
}

func cloneMessageEmbed(embed *discordgo.MessageEmbed) *discordgo.MessageEmbed {
	if embed == nil {
		return nil
	}
	raw, err := json.Marshal(embed)
	if err != nil {
		copy := *embed
		return &copy
	}
	var cloned discordgo.MessageEmbed
	if err := json.Unmarshal(raw, &cloned); err != nil {
		copy := *embed
		return &copy
	}
	return &cloned
}

func baseEmbedForRender(embed *discordgo.MessageEmbed) *discordgo.MessageEmbed {
	cloned := cloneMessageEmbed(embed)
	if cloned != nil {
		cloned.Image = nil
	}
	return cloned
}

func renderTargetKey(target slashRenderTarget) string {
	switch target.Kind {
	case slashRenderTargetOriginal:
		if target.AppID == "" || target.Token == "" {
			return ""
		}
		return "original:" + target.AppID + ":" + target.Token
	case slashRenderTargetChannel:
		if target.ChannelID == "" || target.MessageID == "" {
			return ""
		}
		return "channel:" + target.ChannelID + ":" + target.MessageID
	default:
		return ""
	}
}

func (d *Discord) cleanupSlashRenderStateLocked(now time.Time) {
	for key, state := range d.renderState {
		if state == nil || state.ExpiresAt.Before(now) {
			delete(d.renderState, key)
		}
	}
}

func (d *Discord) cleanupSlashMapCacheLocked(now time.Time) {
	for key, entry := range d.mapCache {
		switch {
		case entry.URL != "" && (!entry.ExpiresAt.After(now)):
			delete(d.mapCache, key)
		case entry.URL == "" && !entry.RetryAfter.IsZero() && !entry.RetryAfter.After(now):
			delete(d.mapCache, key)
		}
	}
}

func (d *Discord) slashMapSuccessTTL(mapReq *slashMapRequest) time.Duration {
	if mapReq == nil {
		return 0
	}
	switch mapReq.Kind {
	case "location":
		return slashLocationMapTTL
	case "area":
		return slashAreaMapSuccessTTL
	default:
		return slashLocationMapTTL
	}
}

func channelRenderTarget(channelID, messageID string) slashRenderTarget {
	return slashRenderTarget{
		Kind:      slashRenderTargetChannel,
		ChannelID: channelID,
		MessageID: messageID,
	}
}

func originalInteractionRenderTarget(i *discordgo.InteractionCreate, message *discordgo.Message) slashRenderTarget {
	target := slashRenderTarget{Kind: slashRenderTargetOriginal}
	if i == nil || i.Interaction == nil {
		return target
	}
	target.AppID = i.Interaction.AppID
	target.Token = i.Interaction.Token
	if message != nil {
		target.MessageID = message.ID
		target.ChannelID = message.ChannelID
	}
	return target
}

func (d *Discord) captureSlashRender(target slashRenderTarget, mapReq *slashMapRequest, embed *discordgo.MessageEmbed) int64 {
	if d == nil || renderTargetKey(target) == "" || embed == nil {
		return 0
	}
	d.ensureSlashMapState()
	mapKey := ""
	if mapReq != nil {
		mapKey = mapReq.Key
	}
	now := time.Now()
	d.renderMu.Lock()
	defer d.renderMu.Unlock()
	d.cleanupSlashRenderStateLocked(now)
	d.renderSeq++
	d.renderState[renderTargetKey(target)] = &slashRenderState{
		Revision:  d.renderSeq,
		MapKey:    mapKey,
		Target:    target,
		BaseEmbed: baseEmbedForRender(embed),
		ExpiresAt: now.Add(slashRenderStateTTL),
	}
	return d.renderSeq
}

func (d *Discord) renderSnapshotsForMapKey(mapKey string) []slashRenderSnapshot {
	if d == nil || mapKey == "" {
		return nil
	}
	d.ensureSlashMapState()
	now := time.Now()
	d.renderMu.Lock()
	defer d.renderMu.Unlock()
	d.cleanupSlashRenderStateLocked(now)
	out := []slashRenderSnapshot{}
	for _, state := range d.renderState {
		if state == nil || state.MapKey != mapKey {
			continue
		}
		out = append(out, slashRenderSnapshot{
			Target:   state.Target,
			Revision: state.Revision,
		})
	}
	return out
}

func (d *Discord) prepareSlashMapPatch(target slashRenderTarget, mapKey string, revision int64, url string) (*slashRenderPatch, bool) {
	if d == nil || renderTargetKey(target) == "" || mapKey == "" || url == "" {
		return nil, false
	}
	d.ensureSlashMapState()
	now := time.Now()
	d.renderMu.Lock()
	defer d.renderMu.Unlock()
	d.cleanupSlashRenderStateLocked(now)
	state := d.renderState[renderTargetKey(target)]
	if state == nil || state.Revision != revision || state.MapKey != mapKey || state.BaseEmbed == nil {
		return nil, false
	}
	embed := cloneMessageEmbed(state.BaseEmbed)
	if embed == nil {
		return nil, false
	}
	embed.Image = &discordgo.MessageEmbedImage{URL: url}
	patch := &slashRenderPatch{Target: target}
	switch target.Kind {
	case slashRenderTargetOriginal:
		patch.WebhookEdit = &discordgo.WebhookEdit{
			Embeds: &[]*discordgo.MessageEmbed{embed},
		}
	case slashRenderTargetChannel:
		patch.ChannelEdit = &discordgo.MessageEdit{
			ID:      target.MessageID,
			Channel: target.ChannelID,
			Embeds:  &[]*discordgo.MessageEmbed{embed},
		}
	default:
		return nil, false
	}
	return patch, true
}

func (d *Discord) slashMapCacheStatus(mapReq *slashMapRequest) (string, bool) {
	if d == nil || mapReq == nil {
		return "", false
	}
	d.ensureSlashMapState()
	now := time.Now()
	d.mapMu.Lock()
	defer d.mapMu.Unlock()
	d.cleanupSlashMapCacheLocked(now)
	entry, ok := d.mapCache[mapReq.Key]
	if !ok || entry.URL == "" || !entry.ExpiresAt.After(now) {
		delete(d.mapCache, mapReq.Key)
		return "", false
	}
	return entry.URL, true
}

func (d *Discord) applySlashMapImage(embed *discordgo.MessageEmbed, mapReq *slashMapRequest) {
	if embed == nil || d == nil {
		return
	}
	if mapReq == nil {
		if d != nil && d.manager != nil && d.manager.cfg != nil {
			if fallback := fallbackStaticMap(d.manager.cfg); fallback != "" {
				embed.Image = &discordgo.MessageEmbedImage{URL: fallback}
			}
		}
		return
	}
	if !d.slashMapsEnabled() {
		if d != nil && d.manager != nil && d.manager.cfg != nil {
			if fallback := fallbackStaticMap(d.manager.cfg); fallback != "" {
				embed.Image = &discordgo.MessageEmbedImage{URL: fallback}
			}
		}
		return
	}
	if url, ok := d.slashMapCacheStatus(mapReq); ok {
		embed.Image = &discordgo.MessageEmbedImage{URL: url}
		if logger := logging.Get().Discord; logger != nil {
			logger.Debugf("Slash map cache hit (%s)", mapReq.Key)
		}
		return
	}
	if logger := logging.Get().Discord; logger != nil {
		logger.Debugf("Slash map cache miss (%s)", mapReq.Key)
	}
}

func (d *Discord) queueSlashMapResolution(s *discordgo.Session, mapReq *slashMapRequest) {
	if d == nil || mapReq == nil || !d.slashMapsEnabled() {
		return
	}
	d.ensureSlashMapState()
	if !d.tryQueueMapJob(mapReq) {
		return
	}
	go d.resolveSlashMapAsync(s, mapReq)
}

func (d *Discord) tryQueueMapJob(mapReq *slashMapRequest) bool {
	now := time.Now()
	d.mapMu.Lock()
	defer d.mapMu.Unlock()
	d.cleanupSlashMapCacheLocked(now)
	if entry, ok := d.mapCache[mapReq.Key]; ok {
		if entry.URL != "" && entry.ExpiresAt.After(now) {
			return false
		}
		if entry.RetryAfter.After(now) {
			if logger := logging.Get().Discord; logger != nil {
				logger.Debugf("Slash map backoff active (%s)", mapReq.Key)
			}
			return false
		}
	}
	if _, ok := d.mapJobs[mapReq.Key]; ok {
		if logger := logging.Get().Discord; logger != nil {
			logger.Debugf("Slash map job reused (%s)", mapReq.Key)
		}
		return false
	}
	d.mapJobs[mapReq.Key] = &slashMapJob{StartedAt: now}
	return true
}

func (d *Discord) resolveSlashMapAsync(s *discordgo.Session, mapReq *slashMapRequest) {
	if d == nil || mapReq == nil {
		return
	}
	start := time.Now()
	d.ensureSlashMapState()
	url, err := d.mapGenerator.Generate(mapReq)
	now := time.Now()

	d.mapMu.Lock()
	delete(d.mapJobs, mapReq.Key)
	d.cleanupSlashMapCacheLocked(now)
	if err != nil || strings.TrimSpace(url) == "" {
		d.mapCache[mapReq.Key] = slashMapCacheEntry{
			RetryAfter: now.Add(slashMapFailureBackoff),
		}
		d.mapMu.Unlock()
		if logger := logging.Get().Discord; logger != nil {
			if err != nil {
				logger.Warnf("Slash map generation failed (%s): %v", mapReq.Key, err)
			} else {
				logger.Warnf("Slash map generation failed (%s): empty result", mapReq.Key)
			}
		}
		return
	}
	url = strings.TrimSpace(url)
	d.mapCache[mapReq.Key] = slashMapCacheEntry{
		URL:       url,
		ExpiresAt: now.Add(d.slashMapSuccessTTL(mapReq)),
	}
	d.mapMu.Unlock()

	if logger := logging.Get().Discord; logger != nil {
		level := logging.LevelDebug
		if d.manager != nil {
			level = logging.TimingLevel(d.manager.cfg)
		}
		logger.Logf(level, "Slash map resolved (%s) in %d ms", mapReq.Key, time.Since(start).Milliseconds())
	}
	d.applyResolvedSlashMap(s, mapReq, url)
}

func (d *Discord) applyResolvedSlashMap(s *discordgo.Session, mapReq *slashMapRequest, url string) {
	if d == nil || mapReq == nil || url == "" {
		return
	}
	session := s
	if session == nil {
		session = d.session
	}
	if session == nil {
		return
	}
	snapshots := d.renderSnapshotsForMapKey(mapReq.Key)
	for _, snapshot := range snapshots {
		patch, ok := d.prepareSlashMapPatch(snapshot.Target, mapReq.Key, snapshot.Revision, url)
		if !ok {
			if logger := logging.Get().Discord; logger != nil {
				logger.Debugf("Discarded stale slash map result (%s) for %s", mapReq.Key, renderTargetKey(snapshot.Target))
			}
			continue
		}
		var err error
		switch patch.Target.Kind {
		case slashRenderTargetOriginal:
			_, err = session.WebhookMessageEdit(patch.Target.AppID, patch.Target.Token, "@original", patch.WebhookEdit)
		case slashRenderTargetChannel:
			_, err = session.ChannelMessageEditComplex(patch.ChannelEdit)
		default:
			continue
		}
		if err != nil {
			if logger := logging.Get().Discord; logger != nil {
				logger.Warnf("Slash map patch failed (%s): %v", mapReq.Key, err)
			}
		}
	}
}

func messageRefFromInteractionMessage(message *discordgo.Message) slashRenderTarget {
	if message == nil {
		return slashRenderTarget{}
	}
	return channelRenderTarget(message.ChannelID, message.ID)
}

func (d *Discord) clearSlashRenderTarget(target slashRenderTarget) {
	if d == nil || renderTargetKey(target) == "" {
		return
	}
	d.ensureSlashMapState()
	d.renderMu.Lock()
	defer d.renderMu.Unlock()
	delete(d.renderState, renderTargetKey(target))
}

func (d *Discord) clearSlashRenderMessage(message *discordgo.Message) {
	if d == nil || message == nil || message.ID == "" {
		return
	}
	d.ensureSlashMapState()
	d.renderMu.Lock()
	defer d.renderMu.Unlock()
	for key, state := range d.renderState {
		if state == nil {
			continue
		}
		if state.Target.MessageID == message.ID {
			delete(d.renderState, key)
		}
	}
}

func (d *Discord) registerSlashRenderRef(s *discordgo.Session, target slashRenderTarget, mapReq *slashMapRequest, embed *discordgo.MessageEmbed) {
	if renderTargetKey(target) == "" {
		return
	}
	d.captureSlashRender(target, mapReq, embed)
	d.queueSlashMapResolution(s, mapReq)
}

func (d *Discord) registerSlashRender(s *discordgo.Session, message *discordgo.Message, mapReq *slashMapRequest, embed *discordgo.MessageEmbed) {
	if message == nil || embed == nil {
		return
	}
	d.registerSlashRenderRef(s, messageRefFromInteractionMessage(message), mapReq, embed)
}

func (d *Discord) registerSlashInteractionRender(s *discordgo.Session, i *discordgo.InteractionCreate, message *discordgo.Message, mapReq *slashMapRequest, embed *discordgo.MessageEmbed) {
	if message == nil || embed == nil {
		return
	}
	target := originalInteractionRenderTarget(i, message)
	if renderTargetKey(target) == "" {
		d.registerSlashRender(s, message, mapReq, embed)
		return
	}
	d.registerSlashRenderRef(s, target, mapReq, embed)
}

func (d *Discord) registerSuccessfulSlashRender(s *discordgo.Session, message *discordgo.Message, err error, mapReq *slashMapRequest, embed *discordgo.MessageEmbed) bool {
	if err != nil || message == nil || embed == nil {
		return false
	}
	d.registerSlashRender(s, message, mapReq, embed)
	return true
}

func (d *Discord) registerSuccessfulSlashInteractionRender(s *discordgo.Session, i *discordgo.InteractionCreate, message *discordgo.Message, err error, mapReq *slashMapRequest, embed *discordgo.MessageEmbed) bool {
	if err != nil || message == nil || embed == nil {
		return false
	}
	d.registerSlashInteractionRender(s, i, message, mapReq, embed)
	return true
}
