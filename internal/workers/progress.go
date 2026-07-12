package workers

import (
	"sync"
	"time"
)

type TaskStatus string

const (
	TaskIdle      TaskStatus = "idle"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
)

type TaskProgress struct {
	Status     TaskStatus `json:"status"`
	Current    int        `json:"current"`
	Total      int        `json:"total"`
	Percentage float64    `json:"percentage"`
	Message    string     `json:"message"`
	StartedAt  *time.Time `json:"started_at"`
	UpdatedAt  *time.Time `json:"updated_at"`
	Duration   string     `json:"duration"`
}

type Progress struct {
	mu     sync.RWMutex
	Scan   TaskProgress
	Scrape TaskProgress
}

func NewProgress() *Progress {
	return &Progress{}
}

func (p *Progress) SetScanRunning(total int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	p.Scan = TaskProgress{
		Status:    TaskRunning,
		Current:   0,
		Total:     total,
		Percentage: 0,
		Message:   "scanning...",
		StartedAt: &now,
		UpdatedAt: &now,
	}
}

func (p *Progress) UpdateScanProgress(current, total int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Scan.Status != TaskRunning {
		return
	}
	now := time.Now()
	p.Scan.Current = current
	p.Scan.Total = total
	if total > 0 {
		p.Scan.Percentage = float64(current) / float64(total) * 100
	}
	p.Scan.UpdatedAt = &now
}

func (p *Progress) SetScanCompleted(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	p.Scan.Status = TaskCompleted
	p.Scan.Current = p.Scan.Total
	p.Scan.Percentage = 100
	p.Scan.Message = message
	p.Scan.UpdatedAt = &now
	if p.Scan.StartedAt != nil {
		p.Scan.Duration = now.Sub(*p.Scan.StartedAt).Round(time.Millisecond).String()
	}
}

func (p *Progress) ResetScan() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Scan = TaskProgress{}
}

func (p *Progress) SetScrapeRunning(total int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	p.Scrape = TaskProgress{
		Status:    TaskRunning,
		Current:   0,
		Total:     total,
		Percentage: 0,
		Message:   "scraping...",
		StartedAt: &now,
		UpdatedAt: &now,
	}
}

func (p *Progress) UpdateScrapeProgress(current, total int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Scrape.Status != TaskRunning {
		return
	}
	now := time.Now()
	p.Scrape.Current = current
	p.Scrape.Total = total
	if total > 0 {
		p.Scrape.Percentage = float64(current) / float64(total) * 100
	}
	p.Scrape.UpdatedAt = &now
}

func (p *Progress) SetScrapeCompleted(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	p.Scrape.Status = TaskCompleted
	p.Scrape.Current = p.Scrape.Total
	p.Scrape.Percentage = 100
	p.Scrape.Message = message
	p.Scrape.UpdatedAt = &now
	if p.Scrape.StartedAt != nil {
		p.Scrape.Duration = now.Sub(*p.Scrape.StartedAt).Round(time.Millisecond).String()
	}
}

func (p *Progress) ResetScrape() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Scrape = TaskProgress{}
}

func (p *Progress) GetStatus() map[string]TaskProgress {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return map[string]TaskProgress{
		"scan":   p.Scan,
		"scrape": p.Scrape,
	}
}
