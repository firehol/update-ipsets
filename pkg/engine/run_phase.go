package engine

type RunPhase string

const (
	RunPhaseUnknown   RunPhase = ""
	RunPhasePreflight RunPhase = "preflight"
	RunPhaseSources   RunPhase = "sources"
	RunPhaseGeoIP     RunPhase = "geoip"
	RunPhaseBogons    RunPhase = "bogons"
	RunPhaseCritical  RunPhase = "critical_infrastructure"
	RunPhaseASN       RunPhase = "asn"
	RunPhaseEntities  RunPhase = "entities"
	RunPhaseMetadata  RunPhase = "metadata"
	RunPhaseInsights  RunPhase = "insights"
	RunPhasePublish   RunPhase = "publish"
)

func (p RunPhase) Valid() bool {
	return p != RunPhaseUnknown
}
