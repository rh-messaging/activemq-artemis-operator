package namespaces

type WatchOptions struct {
	watchAll  bool
	watchList []string
}

func (watch *WatchOptions) SetWatchAll(watchAll bool) {
	watch.watchAll = watchAll
}

func (watch *WatchOptions) SetWatchList(watchList []string) {
	watch.watchList = watchList
}

func (watch *WatchOptions) SetWatchNamespace(namespace string) {
	watch.watchList = []string{namespace}
}

func (watch *WatchOptions) Match(namespace string) bool {
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
