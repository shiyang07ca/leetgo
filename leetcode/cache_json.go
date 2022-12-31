//go:build !cache_sqlite

package leetcode

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/j178/leetgo/config"
	"github.com/j178/leetgo/utils"
)

type cache struct {
	path     string
	once     sync.Once
	slugs    map[string]*questionRecord
	frontIds map[string]*questionRecord
}

func newCache(path string) QuestionsCache {
	return &cache{path: config.Get().LeetCodeCacheFile()}
}

func (c *cache) doLoad() error {
	c.slugs = make(map[string]*questionRecord)
	c.frontIds = make(map[string]*questionRecord)

	var records []questionRecord
	if _, err := os.Stat(c.path); errors.Is(err, os.ErrNotExist) {
		return err
	}
	s, err := os.ReadFile(c.path)
	if err != nil {
		return err
	}
	err = json.Unmarshal(s, &records)
	if err != nil {
		return err
	}
	for _, r := range records {
		r := r
		c.slugs[r.Slug] = &r
		c.frontIds[r.FrontendId] = &r
	}
	return nil
}

func (c *cache) load() {
	c.once.Do(
		func() {
			err := c.doLoad()
			if err != nil {
				hclog.L().Warn("failed to load cache, try updating with `leetgo cache update`")
				return
			}
			c.checkUpdateTime()
		},
	)
}

func (c *cache) checkUpdateTime() {
	stat, err := os.Stat(c.path)
	if os.IsNotExist(err) {
		return
	}
	if time.Since(stat.ModTime()) >= 14*24*time.Hour {
		hclog.L().Warn("cache is too old, try updating with `leetgo cache update`")
	}
}

func (c *cache) Update(client Client) error {
	err := utils.CreateIfNotExists(c.path, false)
	if err != nil {
		return err
	}

	all, err := client.GetAllQuestions()
	if err != nil {
		return err
	}
	questions := make([]questionRecord, 0, len(all))
	for _, q := range all {
		tags := make([]string, 0, len(q.TopicTags))
		for _, t := range q.TopicTags {
			tags = append(tags, t.Slug)
		}
		questions = append(
			questions, questionRecord{
				FrontendId: q.QuestionFrontendId,
				Slug:       q.TitleSlug,
				Title:      q.Title,
				CnTitle:    q.TranslatedTitle,
				Difficulty: q.Difficulty,
				Tags:       tags,
				PaidOnly:   q.IsPaidOnly,
			},
		)
	}
	f, err := os.Create(c.path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "\t")
	err = enc.Encode(questions)
	if err != nil {
		return err
	}
	hclog.L().Info("cache updated", "path", c.path)
	return nil
}

func (c *cache) GetBySlug(slug string) *questionRecord {
	c.load()
	return c.slugs[slug]
}

func (c *cache) GetById(id string) *questionRecord {
	c.load()
	return c.frontIds[id]
}