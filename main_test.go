package main

import (
	"reflect"
	"testing"
)

func TestDifference(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want []string
	}{
		{
			name: "basic difference",
			a:    []string{"1", "2", "3"},
			b:    []string{"2", "3", "4"},
			want: []string{"1"},
		},
		{
			name: "no difference",
			a:    []string{"1", "2", "3"},
			b:    []string{"1", "2", "3"},
			want: []string{},
		},
		{
			name: "empty a",
			a:    []string{},
			b:    []string{"1", "2"},
			want: []string{},
		},
		{
			name: "empty b",
			a:    []string{"1", "2"},
			b:    []string{},
			want: []string{"1", "2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := difference(tt.a, tt.b)
			// 对于空切片的情况，需要特殊处理
			if len(tt.want) == 0 && len(got) == 0 {
				return // 测试通过
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("difference() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIntersection(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want []string
	}{
		{
			name: "basic intersection",
			a:    []string{"1", "2", "3"},
			b:    []string{"2", "3", "4"},
			want: []string{"2", "3"},
		},
		{
			name: "no intersection",
			a:    []string{"1", "2"},
			b:    []string{"3", "4"},
			want: []string{},
		},
		{
			name: "empty a",
			a:    []string{},
			b:    []string{"1", "2"},
			want: []string{},
		},
		{
			name: "empty b",
			a:    []string{"1", "2"},
			b:    []string{},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := intersection(tt.a, tt.b)
			// 对于空切片的情况，需要特殊处理
			if len(tt.want) == 0 && len(got) == 0 {
				return // 测试通过
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("intersection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatLabels(t *testing.T) {
	// 创建一个模拟的 PodController
	pc := &PodController{}

	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{
			name:   "single label",
			labels: map[string]string{"app": "test"},
			want:   "app=test",
		},
		{
			name:   "empty labels",
			labels: map[string]string{},
			want:   "",
		},
		{
			name:   "nil labels",
			labels: nil,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pc.formatLabels(tt.labels)
			// 对于多个标签的情况，由于map的遍历顺序不确定，我们需要特殊处理
			if len(tt.labels) <= 1 {
				if got != tt.want {
					t.Errorf("formatLabels() = %v, want %v", got, tt.want)
				}
			} else {
				// 对于多个标签，检查是否包含所有期望的键值对
				for k, v := range tt.labels {
					expected := k + "=" + v
					if !contains(got, expected) {
						t.Errorf("formatLabels() = %v, should contain %v", got, expected)
					}
				}
			}
		})
	}
}

// 辅助函数：检查字符串是否包含子字符串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				indexOfSubstring(s, substr) >= 0)))
}

// 简单的子字符串查找
func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// 基准测试
func BenchmarkDifference(b *testing.B) {
	a := []string{"1", "2", "3", "4", "5"}
	sliceB := []string{"3", "4", "5", "6", "7"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = difference(a, sliceB)
	}
}

func BenchmarkIntersection(b *testing.B) {
	a := []string{"1", "2", "3", "4", "5"}
	sliceB := []string{"3", "4", "5", "6", "7"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = intersection(a, sliceB)
	}
}
