package alerts

import (
	"strconv"
	"strings"

	pb "github.com/obsidianstack/obsidianstack/gen/obsidian/v1"
)

// evalCondition evaluates a rule condition string against a PipelineSnapshot.
//
// Supported expressions (field operator value):
//
//	drop_pct > 10
//	strength_score < 60
//	throughput < 100
//	uptime_pct < 99
//	latency_p95_ms > 500
//	latency_p99_ms > 1000
//	state == critical
//	state == degraded
//	cert_days_left < 14
//
// Returns (fires bool, triggering value float64).
// Returns (false, 0) if the expression cannot be parsed or the field is unknown.
func evalCondition(cond string, snap *pb.PipelineSnapshot) (bool, float64) {
	parts := strings.Fields(cond)
	if len(parts) != 3 {
		return false, 0
	}
	field, op, rhs := parts[0], parts[1], parts[2]

	switch field {
	case "state":
		if op == "==" {
			return snap.State == rhs, 0
		}
		return false, 0

	case "cert_days_left":
		threshold, err := strconv.ParseFloat(rhs, 64)
		if err != nil {
			return false, 0
		}
		for _, c := range snap.Certs {
			v := float64(c.DaysLeft)
			if compareFloat(v, op, threshold) {
				return true, v
			}
		}
		return false, 0

	default:
		v := numericField(field, snap)
		threshold, err := strconv.ParseFloat(rhs, 64)
		if err != nil {
			return false, 0
		}
		return compareFloat(v, op, threshold), v
	}
}

// numericField maps a field name to its value in the snapshot.
func numericField(field string, snap *pb.PipelineSnapshot) float64 {
	switch field {
	case "drop_pct":
		return snap.DropPct
	case "strength_score":
		return snap.StrengthScore
	case "throughput":
		return snap.ThroughputPerMin
	case "uptime_pct":
		return snap.UptimePct
	case "latency_p95_ms":
		return snap.LatencyP95Ms
	case "latency_p99_ms":
		return snap.LatencyP99Ms
	default:
		return 0
	}
}

// compareFloat applies a comparison operator to two float64 values.
func compareFloat(v float64, op string, threshold float64) bool {
	switch op {
	case ">":
		return v > threshold
	case ">=":
		return v >= threshold
	case "<":
		return v < threshold
	case "<=":
		return v <= threshold
	case "==":
		return v == threshold
	default:
		return false
	}
}
