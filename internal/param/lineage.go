// Package param 负责参数集谱系验证：版本链无环、基线存在、
// 版本语义递增。它是"参数版本错配"归因判定的核心依据。
package param

import (
	"fmt"
	"sort"

	"task283-kemfail/internal/model"
)

// Lineage 表示一条从基线到目标参数集的完整谱系。
type Lineage struct {
	Sets    []model.ParamSet // 从基线（索引 0）到目标（末尾）的有序链
	Cyclic  bool             // 谱系是否成环
	HasBase bool             // 是否存在基线参数
	Depth   int              // 深度（从基线起 0 基）
}

// Resolve 从参数集出发沿 ParentID 回溯，构建谱系并检查不变量。
//
// 不变量：
//  1. 无环（行走步数超过参数总数即判环）；
//  2. 存在基线（链尾 ParentID 为空时，其 IsBaseline 必须为 true）；
//  3. 版本语义递增（子版本号 >= 父版本号，数字段比较）。
func Resolve(get func(id int64) (*model.ParamSet, error), startID int64) (*Lineage, error) {
	chain := []model.ParamSet{}
	seen := map[int64]bool{}
	cur := startID
	for cur != 0 {
		if seen[cur] {
			return nil, model.ErrParamLineageCycle
		}
		seen[cur] = true
		p, err := get(cur)
		if err != nil {
			return nil, err
		}
		chain = append(chain, *p)
		if p.ParentID == nil {
			break
		}
		cur = *p.ParentID
	}
	// 反转为基线在前
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	lin := &Lineage{Sets: chain, Depth: len(chain) - 1}
	if len(chain) == 0 {
		return lin, nil
	}
	// 链首必须是基线
	if chain[0].IsBaseline {
		lin.HasBase = true
	} else {
		lin.HasBase = false
	}
	// 版本递增检查
	for i := 1; i < len(chain); i++ {
		if versionLess(chain[i].Version, chain[i-1].Version) {
			return nil, model.ErrParamVersionRegression
		}
	}
	return lin, nil
}

// Baseline 返回谱系的基线参数；缺基线时返回 ErrParamMissingBaseline。
func (l *Lineage) Baseline() (*model.ParamSet, error) {
	if len(l.Sets) == 0 || !l.HasBase {
		return nil, model.ErrParamMissingBaseline
	}
	return &l.Sets[0], nil
}

// Tip 返回谱系末端（最新）参数。
func (l *Lineage) Tip() *model.ParamSet {
	if len(l.Sets) == 0 {
		return nil
	}
	return &l.Sets[len(l.Sets)-1]
}

// Describe 生成谱系的人类可读描述，供候选证据使用。
func (l *Lineage) Describe() string {
	if len(l.Sets) == 0 {
		return "empty lineage"
	}
	names := make([]string, 0, len(l.Sets))
	for i := range l.Sets {
		names = append(names, fmt.Sprintf("%s@%s", l.Sets[i].Name, l.Sets[i].Version))
	}
	return fmt.Sprintf("%s (depth %d)", joinNames(names), l.Depth)
}

func joinNames(ns []string) string {
	out := ""
	for i, n := range ns {
		if i > 0 {
			out += " -> "
		}
		out += n
	}
	return out
}

// versionLess 语义比较版本号：按数字段逐段比较；任意段数字更大者版本更高。
// 非数字段按字典序比较。返回 a < b。
func versionLess(a, b string) bool {
	return compareVersion(a, b) < 0
}

// compareVersion 返回 -1/0/1（a < b / a == b / a > b）。
func compareVersion(a, b string) int {
	sa := splitVersion(a)
	sb := splitVersion(b)
	n := len(sa)
	if len(sb) > n {
		n = len(sb)
	}
	for i := 0; i < n; i++ {
		var ai, bi string
		if i < len(sa) {
			ai = sa[i]
		}
		if i < len(sb) {
			bi = sb[i]
		}
		if ai == bi {
			continue
		}
		an, aok := asNum(ai)
		bn, bok := asNum(bi)
		switch {
		case aok && bok:
			if an < bn {
				return -1
			}
			if an > bn {
				return 1
			}
		case aok:
			return 1 // 数字段大于非数字段（1.10 > 1.9a）
		case bok:
			return -1
		default:
			if ai < bi {
				return -1
			}
			return 1
		}
	}
	if len(sa) < len(sb) {
		return -1
	}
	if len(sa) > len(sb) {
		return 1
	}
	return 0
}

func splitVersion(v string) []string {
	out := []string{}
	cur := ""
	for _, c := range v {
		if c == '.' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(c)
	}
	out = append(out, cur)
	return out
}

func asNum(s string) (int, bool) {
	if s == "" {
		return 0, true
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n, true
}

// SortByVersion 按版本语义排序参数集（升序），用于基线锁定与比对。
func SortByVersion(sets []model.ParamSet) {
	sort.Slice(sets, func(i, j int) bool {
		return compareVersion(sets[i].Version, sets[j].Version) < 0
	})
}
