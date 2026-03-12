package webhook

type alertTarget struct {
	ID       string
	Type     string
	Name     string
	Language string
	Lat      float64
	Lon      float64
	Areas    []string
	Profile  int
	Template string
	Platform string
}

type alertMatch struct {
	Target alertTarget
	Row    map[string]any
}

type locationInfo struct {
	Lat          float64
	Lon          float64
	Areas        []string
	Restrictions []string
}
