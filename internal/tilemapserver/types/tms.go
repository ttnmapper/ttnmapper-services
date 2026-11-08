package types

type Sample struct {
	X              int
	Y              int
	MaxBucketIndex int
}

// ByRssi implements sort.Interface based on the Rssi field.
type ByRssi []Sample

func (a ByRssi) Len() int           { return len(a) }
func (a ByRssi) Less(i, j int) bool { return a[i].MaxBucketIndex > a[j].MaxBucketIndex }
func (a ByRssi) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
