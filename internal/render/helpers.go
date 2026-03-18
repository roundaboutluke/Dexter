package render

import (
	"sync"

	"github.com/aymerick/raymond"

	"dexter/internal/config"
	"dexter/internal/data"
	"dexter/internal/i18n"
)

var (
	initOnce    sync.Once
	cfg         *config.Config
	gameData    *data.GameData
	translator  *i18n.Factory
	altLanguage string
	customMaps  []map[string]any
	customEmoji map[string]map[string]string
)

// Init registers helpers and caches data for rendering.
func Init(root string, cfgIn *config.Config, dataIn *data.GameData, i18nFactory *i18n.Factory) {
	initOnce.Do(func() {
		cfg = cfgIn
		gameData = dataIn
		translator = i18nFactory
		if cfg != nil {
			if lang, ok := cfg.GetString("locale.language"); ok {
				altLanguage = lang
			}
		}
		customMaps = loadCustomMaps(root)
		customEmoji = loadCustomEmoji(root)
		_ = RegisterPartials(root)

		raymond.RegisterHelper("round", roundHelper)
		raymond.RegisterHelper("eq", eqHelper)
		raymond.RegisterHelper("is", isHelper)
		raymond.RegisterHelper("isnt", isntHelper)
		raymond.RegisterHelper("ne", isntHelper)
		raymond.RegisterHelper("neq", isntHelper)
		raymond.RegisterHelper("notEq", isntHelper)
		raymond.RegisterHelper("notEquals", isntHelper)
		raymond.RegisterHelper("equals", eqHelper)
		raymond.RegisterHelper("compare", compareHelper)
		raymond.RegisterHelper("gt", gtHelper)
		raymond.RegisterHelper("gte", gteHelper)
		raymond.RegisterHelper("lt", ltHelper)
		raymond.RegisterHelper("lte", lteHelper)
		raymond.RegisterHelper("forEach", forEachHelper)
		raymond.RegisterHelper("minus", minusHelper)
		raymond.RegisterHelper("or", orHelper)
		raymond.RegisterHelper("and", andHelper)
		raymond.RegisterHelper("isFalsey", isFalseyHelper)
		raymond.RegisterHelper("uppercase", uppercaseHelper)
		raymond.RegisterHelper("lowercase", lowercaseHelper)
		raymond.RegisterHelper("upper", uppercaseHelper)
		raymond.RegisterHelper("lower", lowercaseHelper)
		raymond.RegisterHelper("toUpperCase", uppercaseHelper)
		raymond.RegisterHelper("toLowerCase", lowercaseHelper)
		raymond.RegisterHelper("capitalize", capitalizeHelper)
		raymond.RegisterHelper("pvpSlug", pvpSlugHelper)
		raymond.RegisterHelper("ex", exHelper)
		raymond.RegisterHelper("numberFormat", numberFormatHelper)
		raymond.RegisterHelper("pad0", pad0Helper)
		raymond.RegisterHelper("addCommas", addCommasHelper)
		raymond.RegisterHelper("replaceFirst", replaceFirstHelper)
		raymond.RegisterHelper("sum", sumHelper)
		raymond.RegisterHelper("moveName", moveNameHelper)
		raymond.RegisterHelper("moveNameAlt", moveNameAltHelper)
		raymond.RegisterHelper("moveNameEng", moveNameEngHelper)
		raymond.RegisterHelper("moveType", moveTypeHelper)
		raymond.RegisterHelper("moveTypeAlt", moveTypeAltHelper)
		raymond.RegisterHelper("moveTypeEng", moveTypeEngHelper)
		raymond.RegisterHelper("moveEmoji", moveEmojiHelper)
		raymond.RegisterHelper("moveEmojiAlt", moveEmojiAltHelper)
		raymond.RegisterHelper("moveEmojiEng", moveEmojiEngHelper)
		raymond.RegisterHelper("pokemon", pokemonHelper)
		raymond.RegisterHelper("pokemonName", pokemonNameHelper)
		raymond.RegisterHelper("pokemonNameAlt", pokemonNameAltHelper)
		raymond.RegisterHelper("pokemonNameEng", pokemonNameEngHelper)
		raymond.RegisterHelper("pokemonForm", pokemonFormHelper)
		raymond.RegisterHelper("pokemonFormAlt", pokemonFormAltHelper)
		raymond.RegisterHelper("pokemonFormEng", pokemonFormEngHelper)
		raymond.RegisterHelper("translateAlt", translateAltHelper)
		raymond.RegisterHelper("calculateCp", calculateCpHelper)
		raymond.RegisterHelper("pokemonBaseStats", pokemonBaseStatsHelper)
		raymond.RegisterHelper("getEmoji", getEmojiHelper)
		raymond.RegisterHelper("getPowerUpCost", getPowerUpCostHelper)
		raymond.RegisterHelper("map", mapHelper)
		raymond.RegisterHelper("map2", map2Helper)
		raymond.RegisterHelper("concat", concatHelper)
		raymond.RegisterHelper("json", jsonHelper)
		raymond.RegisterHelper("stringify", jsonHelper)
		raymond.RegisterHelper("length", lengthHelper)
		raymond.RegisterHelper("default", defaultHelper)
		raymond.RegisterHelper("not", notHelper)
		raymond.RegisterHelper("contains", containsHelper)
		raymond.RegisterHelper("includes", containsHelper)
		raymond.RegisterHelper("startsWith", startsWithHelper)
		raymond.RegisterHelper("endsWith", endsWithHelper)
		raymond.RegisterHelper("split", splitHelper)
		raymond.RegisterHelper("join", joinHelper)
		raymond.RegisterHelper("replace", replaceHelper)
		raymond.RegisterHelper("replaceAll", replaceHelper)
		raymond.RegisterHelper("add", addHelper)
		raymond.RegisterHelper("subtract", subtractHelper)
		raymond.RegisterHelper("multiply", multiplyHelper)
		raymond.RegisterHelper("divide", divideHelper)
		raymond.RegisterHelper("mod", modHelper)
		raymond.RegisterHelper("ceil", ceilHelper)
		raymond.RegisterHelper("floor", floorHelper)
		raymond.RegisterHelper("abs", absHelper)
		raymond.RegisterHelper("min", minHelper)
		raymond.RegisterHelper("max", maxHelper)
	})
}
