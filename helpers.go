package main

func copyMap(originalMap map[float64]float64) map[float64]float64 {
	newMap := make(map[float64]float64)
	for key, value := range originalMap {
		newMap[key] = value
	}
	return newMap
}

func joinWithLabelsMap(ar []string, labels map[string]string) []string {
	for label := range labels {
		ar = append(ar, label)
	}
	return ar
}

func labelValues(labels map[string]string) []string {
	var values []string
	for _, value := range labels {
		values = append(values, value)
	}
	return values
}
