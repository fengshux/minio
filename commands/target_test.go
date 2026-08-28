package commands

import "testing"

func TestParseTarget(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantCtx    string
		wantBucket string
		wantPath   string
		wantErr    bool
	}{
		{
			name:       "无 context 前缀",
			target:     "my-bucket/file.txt",
			wantBucket: "my-bucket",
			wantPath:   "file.txt",
		},
		{
			name:       "有 context 前缀",
			target:     "prod:my-bucket/file.txt",
			wantCtx:    "prod",
			wantBucket: "my-bucket",
			wantPath:   "file.txt",
		},
		{
			name:       "对象名含冒号，冒号在斜杠之后不算 context",
			target:     "my-bucket/a:b.txt",
			wantBucket: "my-bucket",
			wantPath:   "a:b.txt",
		},
		{
			name:       "context 前缀 + 对象名含冒号",
			target:     "prod:my-bucket/a:b.txt",
			wantCtx:    "prod",
			wantBucket: "my-bucket",
			wantPath:   "a:b.txt",
		},
		{
			name:       "多级路径",
			target:     "dev:my-bucket/photos/2024/img.jpg",
			wantCtx:    "dev",
			wantBucket: "my-bucket",
			wantPath:   "photos/2024/img.jpg",
		},
		{
			name:       "目录前缀",
			target:     "prod:my-bucket/photos/",
			wantCtx:    "prod",
			wantBucket: "my-bucket",
			wantPath:   "photos/",
		},
		{
			name:       "仅 bucket，无 path",
			target:     "my-bucket",
			wantBucket: "my-bucket",
			wantPath:   "",
		},
		{
			name:       "context 前缀 + 仅 bucket",
			target:     "prod:my-bucket",
			wantCtx:    "prod",
			wantBucket: "my-bucket",
			wantPath:   "",
		},
		{
			name:       "bucket 带斜杠但 path 为空",
			target:     "prod:my-bucket/",
			wantCtx:    "prod",
			wantBucket: "my-bucket",
			wantPath:   "",
		},
		{
			name:    "context 名为空",
			target:  ":my-bucket/file.txt",
			wantErr: true,
		},
		{
			name:    "context 前缀后 bucket 为空",
			target:  "prod:",
			wantErr: true,
		},
		{
			name:    "context 前缀后仅有斜杠",
			target:  "prod:/file.txt",
			wantErr: true,
		},
		{
			name:    "空字符串",
			target:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCtx, gotBucket, gotPath, err := parseTarget(tt.target)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseTarget(%q) 期望报错，实际返回 ctx=%q bucket=%q path=%q",
						tt.target, gotCtx, gotBucket, gotPath)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseTarget(%q) 意外报错: %v", tt.target, err)
			}
			if gotCtx != tt.wantCtx {
				t.Errorf("parseTarget(%q) ctx = %q, 期望 %q", tt.target, gotCtx, tt.wantCtx)
			}
			if gotBucket != tt.wantBucket {
				t.Errorf("parseTarget(%q) bucket = %q, 期望 %q", tt.target, gotBucket, tt.wantBucket)
			}
			if gotPath != tt.wantPath {
				t.Errorf("parseTarget(%q) path = %q, 期望 %q", tt.target, gotPath, tt.wantPath)
			}
		})
	}
}
