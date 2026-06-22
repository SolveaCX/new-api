package service

// dueStep 根据注册时间返回当前"最大已到期"的 step(1-4),0 表示还没到 E1。
// createdAt: 用户注册时间戳;delays: step→延迟天数;nowTs: 当前时间戳。
func dueStep(createdAt int64, delays map[int]int, nowTs int64) int {
	ageSeconds := nowTs - createdAt
	if ageSeconds < 0 {
		return 0 // 注册时间在未来(时钟异常),不发
	}
	ageDays := ageSeconds / (24 * 3600)
	result := 0
	for step := 1; step <= 4; step++ {
		d, ok := delays[step]
		if !ok {
			continue
		}
		if ageDays >= int64(d) && step > result {
			result = step
		}
	}
	return result
}
