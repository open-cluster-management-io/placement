package util

import (
	clusterapiv1 "open-cluster-management.io/api/cluster/v1"
	"open-cluster-management.io/placement/pkg/plugins"
)

func NormalizeScore(minScore, maxScore, scale int64, clusters []*clusterapiv1.ManagedCluster, scores map[string]int64) {
	// normalize the score and ensure the value falls in the range between 0 and 100.
	if minScore > maxScore {
		return
	}

	// normalized = (score - min(score)) * 100 / (max(score) - min(score)) * scale
	for _, cluster := range clusters {
		if minScore < maxScore {
			scores[cluster.Name] = scale * int64(float64(plugins.MaxClusterScore)*((float64(scores[cluster.Name]-minScore))/float64(maxScore-minScore)-0.5))
		} else if minScore == maxScore {
			if minScore == 0 {
				scores[cluster.Name] = 0
			} else {
				scores[cluster.Name] = plugins.MaxClusterScore * scale
			}
		}
	}
}

func Min(a, b int64) int64 {
	if a < b {
		return a
	} else {
		return b
	}
}

func Max(a, b int64) int64 {
	if a > b {
		return a
	} else {
		return b
	}
}
