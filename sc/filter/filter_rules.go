/*
 * @Author: gitsrc
 * @Date: 2020-08-05 13:38:32
 * @LastEditors: gitsrc
 * @LastEditTime: 2020-08-19 19:27:32
 * @FilePath: /redis-cluster-proxy/sc/filter/filter_rules.go
 */

package filter

import (
	"gitlab.oneitfarm.com/bifrost/ratelimiter"
)

// IsExistInFilterList :检验key是不是在Filter白名单里面，如果在则进行日志记录
func (filter *Filter) IsExistInFilterList(key string) (ret bool) {
	// 如果关闭了filter功能，则放行所有key的请求
	if !filter.enable {
		return true
	}
	ret = false
	filter.FilterRuleData.RLock()
	defer filter.FilterRuleData.RUnlock()
	if _, ok := filter.FilterRuleData.RuleDataMAP[key]; ok {
		ret = true
		return
	}
	return
}

// GetFilterRule 返回key的策略
func (filter *Filter) GetFilterRule(key string) (limiter *ratelimiter.Limiter, samplingRate uint32, expire int, lengthDrop bool) {
	// 如果关闭了filter功能，则放行所有key的请求
	if !filter.enable {
		return
	}
	filter.FilterRuleData.RLock()
	defer filter.FilterRuleData.RUnlock()
	filterKey, ok := filter.FilterRuleData.RuleDataMAP[key]
	if !ok {
		return
	}
	limiter = filterKey.RateLimiter
	samplingRate = filterKey.SamplingRate
	expire = filterKey.Expire
	lengthDrop = filterKey.Length >= filterKey.WarningLength
	return
}
