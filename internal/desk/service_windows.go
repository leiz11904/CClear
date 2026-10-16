//go:build windows

package desk

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"cclear/internal/space"
	"cclear/internal/syswin"
)

// Service runs the existing read-only disk scan and serves one directory level at a time.
// It does not clean, delete, or change the safety rules.
type Service struct {
	mu       sync.Mutex
	drive    string
	cancel   context.CancelFunc
	running  bool
	ready    bool
	where    string
	seen     int
	report   space.Report
	progress func(files int, where string)
	done     func(Status)
}

func New(progress func(files int, where string), done func(Status)) *Service {
	return &Service{drive: syswin.SystemDrive(), progress: progress, done: done}
}

func (s *Service) Volume() (Volume, error) {
	drive := syswin.SystemDrive()
	total, free, err := syswin.Disk(drive)
	if err != nil {
		return Volume{Drive: drive}, err
	}
	used := uint64(0)
	if total > free {
		used = total - free
	}
	return Volume{Drive: drive, Total: total, Free: free, Used: used}, nil
}

// Start keeps the previous entry point. A running scan is left alone.
func (s *Service) Start() {
	_ = s.StartContext(context.Background())
}

// StartContext runs one read-only scan. It refuses to replace a scan that is already running.
func (s *Service) StartContext(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("磁盘分析已在进行")
	}
	s.running = true
	s.where = ""
	s.seen = 0
	drive := s.drive
	s.mu.Unlock()

	go s.run(ctx, drive)
	return nil
}

// Store keeps a finished or partial scan so other pages can read it later.
func (s *Service) Store(rep space.Report) Status {
	s.mu.Lock()
	s.report = rep
	s.ready = true
	s.running = false
	s.where = ""
	st := s.statusLocked()
	s.mu.Unlock()
	return st
}

func (s *Service) run(ctx context.Context, drive string) {
	defer func() {
		if rec := recover(); rec != nil {
			s.mu.Lock()
			s.running = false
			s.report.Errors = append(s.report.Errors, fmt.Sprint(rec))
			st := s.statusLocked()
			s.mu.Unlock()
			if s.done != nil {
				s.done(st)
			}
		}
	}()
	rep := space.Scan(ctx, drive+`\`, func(files int, where string) {
		s.mu.Lock()
		s.seen = files
		s.where = where
		s.mu.Unlock()
		if s.progress != nil {
			s.progress(files, where)
		}
	})
	s.mu.Lock()
	s.report = rep
	s.ready = true
	s.running = false
	s.where = ""
	st := s.statusLocked()
	s.mu.Unlock()
	if s.done != nil {
		s.done(st)
	}
}

func (s *Service) Cancel() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusLocked()
}

func (s *Service) statusLocked() Status {
	st := Status{
		Running:   s.running,
		Ready:     s.ready,
		Cancelled: s.report.Cancelled,
		Files:     s.report.Files,
		Allocated: s.report.Allocated,
		Note:      s.report.Note,
		Where:     s.where,
	}
	if s.running && s.seen > 0 {
		st.Files = s.seen
	}
	if len(s.report.Errors) > 0 {
		st.Error = strings.Join(s.report.Errors, "\n")
	}
	return st
}

func (s *Service) Level(path string) Level {
	s.mu.Lock()
	tree := s.report.Tree
	s.mu.Unlock()
	return LevelFrom(tree, path)
}

func (s *Service) Types() []TypeRow {
	s.mu.Lock()
	defer s.mu.Unlock()
	return TypesFrom(s.report)
}

func (s *Service) Large() []FileRow {
	s.mu.Lock()
	defer s.mu.Unlock()
	return LargeFrom(s.report, 12)
}

func (s *Service) Reveal(path string) error {
	if path == "" {
		return nil
	}
	return syswin.Reveal(path)
}
