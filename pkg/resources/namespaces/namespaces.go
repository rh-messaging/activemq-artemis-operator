package namespaces

type WatchOptions struct {
	watchAll  bool
	watchList []string
}

var watch WatchOptions = WatchOptions{
	watchAll:  false,
	watchList: []string{},
}

func SetWatchAll(watchAll bool) {
	watch.watchAll = watchAll
}

func SetWatchList(watchList []string) {
	watch.watchList = watchList
}

func SetWatchNamespace(namespace string) {
	watch.watchList = []string{namespace}
}

func Match(namespace string) bool {
	if watch.watchAll {
		return true
	}
	for _, n := range watch.watchList {
		if n == namespace {
			return true
		}
	}
	return false
}
