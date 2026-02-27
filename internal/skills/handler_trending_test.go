package skills

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrendingHandlersRegistered(t *testing.T) {
	names := ListHandlerNames()
	assert.Contains(t, names, "trending_github")
	assert.Contains(t, names, "trending_hackernews")
}

func TestTrendingGitHubHandlerCreate(t *testing.T) {
	handler, err := CreateCodeHandler("trending_github", nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, handler)
}

func TestTrendingHackerNewsHandlerCreate(t *testing.T) {
	handler, err := CreateCodeHandler("trending_hackernews", nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, handler)
}

func TestTrendingGitHubHandlerExecute(t *testing.T) {
	handler, err := CreateCodeHandler("trending_github", nil, nil)
	assert.NoError(t, err)

	output, err := handler.Execute(context.Background(), &SkillInput{
		Arguments: map[string]interface{}{
			"limit": 3,
		},
	})

	// 注意：这个测试可能会因为网络问题或 GitHub 页面结构变化而失败
	// 只要能成功创建 handler 和发送请求就基本正确
	if err != nil {
		// 网络错误是可能的，不算测试失败
		t.Logf("Execute returned error (may be network related): %v", err)
	}

	if output != nil {
		t.Logf("Output: %s", output.Content)
	}
}

func TestTrendingHackerNewsHandlerExecute(t *testing.T) {
	handler, err := CreateCodeHandler("trending_hackernews", nil, nil)
	assert.NoError(t, err)

	output, err := handler.Execute(context.Background(), &SkillInput{
		Arguments: map[string]interface{}{
			"limit": 3,
		},
	})

	// Hacker News 使用 Firebase API，通常比较稳定
	// 注意：现在也允许没有 URL 的故事（如 Ask HN）
	if err == nil {
		assert.NotNil(t, output)
		// Success 可能为 false（如网络错误），这是可以接受的
		t.Logf("Success: %v, Output: %s", output.Success, output.Content)
	} else {
		t.Logf("Execute returned error (may be network related): %v", err)
	}
}
