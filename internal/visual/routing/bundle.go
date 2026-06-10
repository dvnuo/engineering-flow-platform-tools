package routing

import "sort"

func BuildBundles(routes []Route) []RouteBundle {
	byGroup := map[string][]string{}
	for _, route := range routes {
		group := route.PathGroup
		if group == "" {
			continue
		}
		byGroup[group] = append(byGroup[group], route.ID)
	}
	ids := make([]string, 0, len(byGroup))
	for id, members := range byGroup {
		if len(members) < 2 {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]RouteBundle, 0, len(ids))
	for _, id := range ids {
		members := byGroup[id]
		sort.Strings(members)
		out = append(out, RouteBundle{ID: id, PathGroup: id, RouteIDs: members})
	}
	return out
}

func groupRouteIndexes(routes []Route) map[string][]int {
	out := map[string][]int{}
	for i, route := range routes {
		key := route.PathGroup
		if key == "" {
			key = route.ID
		}
		out[key] = append(out[key], i)
	}
	return out
}
