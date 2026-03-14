package command

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"poraclego/internal/i18n"
)

// RegexSet mirrors PoracleJS regex helpers for commands.
type RegexSet struct {
	Name       *regexp.Regexp
	User       *regexp.Regexp
	Channel    *regexp.Regexp
	Guild      *regexp.Regexp
	Form       *regexp.Regexp
	Move       *regexp.Regexp
	Template   *regexp.Regexp
	Gen        *regexp.Regexp
	IV         *regexp.Regexp
	MaxIV      *regexp.Regexp
	Level      *regexp.Regexp
	MaxLevel   *regexp.Regexp
	CP         *regexp.Regexp
	MaxCP      *regexp.Regexp
	Weight     *regexp.Regexp
	MaxWeight  *regexp.Regexp
	Rarity     *regexp.Regexp
	MaxRarity  *regexp.Regexp
	MaxAtk     *regexp.Regexp
	MaxDef     *regexp.Regexp
	MaxSta     *regexp.Regexp
	Atk        *regexp.Regexp
	Def        *regexp.Regexp
	Sta        *regexp.Regexp
	Cap        *regexp.Regexp
	Size       *regexp.Regexp
	MaxSize    *regexp.Regexp
	Great      *regexp.Regexp
	GreatHigh  *regexp.Regexp
	GreatCP    *regexp.Regexp
	Ultra      *regexp.Regexp
	UltraHigh  *regexp.Regexp
	UltraCP    *regexp.Regexp
	Little     *regexp.Regexp
	LittleHigh *regexp.Regexp
	LittleCP   *regexp.Regexp
	Distance   *regexp.Regexp
	Time       *regexp.Regexp
	Amount     *regexp.Regexp
	Stardust   *regexp.Regexp
	Energy     *regexp.Regexp
	Candy      *regexp.Regexp
	Area       *regexp.Regexp
	Language   *regexp.Regexp
	Mon        *regexp.Regexp
	Tue        *regexp.Regexp
	Wed        *regexp.Regexp
	Thu        *regexp.Regexp
	Fri        *regexp.Regexp
	Sat        *regexp.Regexp
	Sun        *regexp.Regexp
	Weekday    *regexp.Regexp
	Weekend    *regexp.Regexp
	MinSpawn   *regexp.Regexp
	LatLon     *regexp.Regexp
}

// NewRegexSet creates command regexes.
func NewRegexSet(factory *i18n.Factory) *RegexSet {
	return &RegexSet{
		Name:       createCommandRegex(factory, "name", "\\S+"),
		User:       createCommandRegex(factory, "user", "-?\\d{1,20}"),
		Channel:    createCommandRegex(factory, "channel", "\\d{1,20}"),
		Guild:      createCommandRegex(factory, "guild", "\\d{1,20}"),
		Form:       createCommandRegex(factory, "form", ".+"),
		Move:       createCommandRegex(factory, "move", ".+"),
		Template:   createCommandRegex(factory, "template", ".+"),
		Gen:        createCommandRegex(factory, "gen", "[1-9]+"),
		IV:         createCommandRegex(factory, "iv", "(?<min>[0-9]{1,3})(\\-(?<max>[0-9]{1,3}))?"),
		MaxIV:      createCommandRegex(factory, "maxiv", "\\d{1,3}"),
		Level:      createCommandRegex(factory, "level", "(?<min>[0-9]{1,2})(\\-(?<max>[0-9]{1,2}))?"),
		MaxLevel:   createCommandRegex(factory, "maxlevel", "\\d{1,2}"),
		CP:         createCommandRegex(factory, "cp", "(?<min>[0-9]{1,5})(\\-(?<max>[0-9]{1,5}))?"),
		MaxCP:      createCommandRegex(factory, "maxcp", "\\d{1,5}"),
		Weight:     createCommandRegex(factory, "weight", "(?<min>[0-9]{1,8})(\\-(?<max>[0-9]{1,8}))?"),
		MaxWeight:  createCommandRegex(factory, "maxweight", "\\d{1,8}"),
		Rarity:     createCommandRegex(factory, "rarity", ".+"),
		MaxRarity:  createCommandRegex(factory, "maxrarity", ".+"),
		MaxAtk:     createCommandRegex(factory, "maxatk", "\\d{1,2}"),
		MaxDef:     createCommandRegex(factory, "maxdef", "\\d{1,2}"),
		MaxSta:     createCommandRegex(factory, "maxsta", "\\d{1,2}"),
		Atk:        createCommandRegex(factory, "atk", "(?<min>[0-9]{1,2})(\\-(?<max>[0-9]{1,2}))?"),
		Def:        createCommandRegex(factory, "def", "(?<min>[0-9]{1,2})(\\-(?<max>[0-9]{1,2}))?"),
		Sta:        createCommandRegex(factory, "sta", "(?<min>[0-9]{1,2})(\\-(?<max>[0-9]{1,2}))?"),
		Cap:        createCommandRegex(factory, "cap", "\\d{1,4}"),
		Size:       createCommandRegex(factory, "size", "(?<min>[0-9a-zA-Z]{1,3})(\\-(?<max>[0-9a-zA-Z]{1,3}))?"),
		MaxSize:    createCommandRegex(factory, "size", "[0-9a-zA-Z]{1,3}"),
		Great:      createCommandRegex(factory, "great", "(?<min>[0-9]{1,4})(\\-(?<max>[0-9]{1,4}))?"),
		GreatHigh:  createCommandRegex(factory, "greathigh", "\\d{1,4}"),
		GreatCP:    createCommandRegex(factory, "greatcp", "\\d{1,5}"),
		Ultra:      createCommandRegex(factory, "ultra", "(?<min>[0-9]{1,4})(\\-(?<max>[0-9]{1,4}))?"),
		UltraHigh:  createCommandRegex(factory, "ultrahigh", "\\d{1,4}"),
		UltraCP:    createCommandRegex(factory, "ultracp", "\\d{1,5}"),
		Little:     createCommandRegex(factory, "little", "(?<min>[0-9]{1,4})(\\-(?<max>[0-9]{1,4}))?"),
		LittleHigh: createCommandRegex(factory, "littlehigh", "\\d{1,4}"),
		LittleCP:   createCommandRegex(factory, "littlecp", "\\d{1,5}"),
		Distance:   createCommandRegex(factory, "d", "[\\d.]{1,}"),
		Time:       createCommandRegex(factory, "t", "\\d{1,4}"),
		Amount:     createCommandRegex(factory, "amount", "\\d{1,8}"),
		Stardust:   createCommandRegex(factory, "stardust", "\\d{1,8}"),
		Energy:     createCommandRegex(factory, "energy", ".+"),
		Candy:      createCommandRegex(factory, "candy", ".+"),
		Area:       createCommandRegex(factory, "area", ".+"),
		Language:   createCommandRegex(factory, "language", ".+"),
		Mon:        createCommandRegex(factory, "mon", "(\\d\\d?)?(:?)(\\d\\d?)?"),
		Tue:        createCommandRegex(factory, "tue", "(\\d\\d?)?(:?)(\\d\\d?)?"),
		Wed:        createCommandRegex(factory, "wed", "(\\d\\d?)?(:?)(\\d\\d?)?"),
		Thu:        createCommandRegex(factory, "thu", "(\\d\\d?)?(:?)(\\d\\d?)?"),
		Fri:        createCommandRegex(factory, "fri", "(\\d\\d?)?(:?)(\\d\\d?)?"),
		Sat:        createCommandRegex(factory, "sat", "(\\d\\d?)?(:?)(\\d\\d?)?"),
		Sun:        createCommandRegex(factory, "sun", "(\\d\\d?)?(:?)(\\d\\d?)?"),
		Weekday:    createCommandRegex(factory, "weekday", "(\\d\\d?)?(:?)(\\d\\d?)?"),
		Weekend:    createCommandRegex(factory, "weekend", "(\\d\\d?)?(:?)(\\d\\d?)?"),
		MinSpawn:   createCommandRegex(factory, "minspawn", "(\\d\\d?)?(:?)(\\d\\d?)?"),
		LatLon:     regexp.MustCompile("^([-+]?(?:[1-8]?\\d(?:\\.\\d+)?|90(?:\\.0+)?)),\\s*([-+]?(?:180(\\.0+)?|(?:(?:1[0-7]\\d)|(?:[1-9]?\\d))(?:\\.\\d+)?))$"),
	}
}

func createCommandRegex(factory *i18n.Factory, commandName, paramMatch string) *regexp.Regexp {
	translated := factory.TranslateCommand(commandName)
	sort.Slice(translated, func(i, j int) bool { return len(translated[i]) > len(translated[j]) })
	expr := strings.Join(translated, "|")
	return regexp.MustCompile(fmt.Sprintf("^(%s):?(%s)", expr, paramMatch))
}
