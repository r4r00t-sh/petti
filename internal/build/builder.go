package build

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"time"

	"github.com/r4r00t-sh/petti/internal/image"
	"github.com/r4r00t-sh/petti/internal/registry"
	"github.com/r4r00t-sh/petti/internal/rootfs"
	"github.com/r4r00t-sh/petti/internal/runtime"
	"github.com/r4r00t-sh/petti/internal/state"
)

// Options configures a build.
type Options struct {
	ContextDir string
	Petfile    string // path relative to context or absolute
	Tag        string // e.g. myapp:latest → localhost/myapp:latest
	Stdout     io.Writer
	Stderr     io.Writer
	NoCache    bool
}

// Result is the built image record.
type Result struct {
	Record image.Record
}

// Builder builds images from Petfiles.
type Builder struct {
	Store *image.Store
}

// New returns a builder using the image store at root (empty → DataRoot).
func New(root string) (*Builder, error) {
	s, err := image.OpenStore(root)
	if err != nil {
		return nil, err
	}
	return &Builder{Store: s}, nil
}

// Build executes the Petfile.
func (b *Builder) Build(ctx context.Context, opts Options) (Result, error) {
	var out Result
	if opts.ContextDir == "" {
		opts.ContextDir = "."
	}
	ctxDir, err := filepath.Abs(opts.ContextDir)
	if err != nil {
		return out, err
	}
	petfile := opts.Petfile
	if petfile == "" {
		petfile = "Petfile"
	}
	// -f is cwd-relative (Docker-like). Fall back to context dir for bare "Petfile".
	if !filepath.IsAbs(petfile) {
		if abs, err := filepath.Abs(petfile); err == nil {
			if _, err := os.Stat(abs); err == nil {
				petfile = abs
			} else {
				petfile = filepath.Join(ctxDir, filepath.Base(petfile))
			}
		}
	}
	insts, err := ParseFile(petfile)
	if err != nil {
		return out, err
	}
	log := opts.Stderr
	if log == nil {
		log = io.Discard
	}
	tag := opts.Tag
	if tag == "" {
		tag = "petti-build:latest"
	}
	ref, err := parseLocalTag(tag)
	if err != nil {
		return out, err
	}

	cfg := registry.ImageConfig{
		Architecture: runtimeGOARCH(),
		OS:           "linux",
		Created:      time.Now().UTC().Format(time.RFC3339),
	}
	cfg.Config.Env = []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
	cfg.RootFS.Type = "layers"

	var layers []image.LayerMeta
	cacheKey := ""

	for i, inst := range insts {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		default:
		}
		_, _ = fmt.Fprintf(log, "Step %d/%d : %s\n", i+1, len(insts), inst.Raw)

		switch inst.Kind {
		case From:
			base, err := b.resolveBase(ctx, inst.Args[0], log)
			if err != nil {
				return out, err
			}
			layers = append([]image.LayerMeta(nil), base.Layers...)
			cfg = base.Config
			if cfg.Config.Env == nil {
				cfg.Config.Env = []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
			}
			cfg.RootFS.Type = "layers"
			cfg.RootFS.DiffIDs = append([]string(nil), base.Config.RootFS.DiffIDs...)
			cacheKey = hashStr("FROM " + inst.Args[0] + " " + base.ID)
			_, _ = fmt.Fprintf(log, " ---> %s\n", short(base.ID))

		case Env:
			for _, a := range inst.Args {
				cfg.Config.Env = upsertEnv(cfg.Config.Env, a)
			}
			cacheKey = hashStr(cacheKey + "\n" + inst.Raw)

		case Workdir:
			cfg.Config.WorkingDir = inst.Args[0]
			cacheKey = hashStr(cacheKey + "\n" + inst.Raw)

		case Cmd:
			cfg.Config.Cmd = append([]string(nil), inst.Args...)
			cacheKey = hashStr(cacheKey + "\n" + inst.Raw)

		case Entrypoint:
			cfg.Config.Entrypoint = append([]string(nil), inst.Args...)
			cacheKey = hashStr(cacheKey + "\n" + inst.Raw)

		case Copy:
			dest := inst.Args[len(inst.Args)-1]
			srcs := inst.Args[:len(inst.Args)-1]
			layerKey := hashStr(cacheKey + "\n" + inst.Raw + "\n" + fileFingerprint(ctxDir, srcs))
			meta, hit, err := b.cachedLayer(layerKey, opts.NoCache)
			if err != nil {
				return out, err
			}
			if hit {
				_, _ = fmt.Fprintf(log, " ---> Using cache\n")
			} else {
				meta, err = b.layerFromCopy(ctxDir, srcs, dest, layerKey, log)
				if err != nil {
					return out, err
				}
			}
			layers = append(layers, meta)
			cfg.RootFS.DiffIDs = append(cfg.RootFS.DiffIDs, meta.DiffID)
			cacheKey = layerKey
			_, _ = fmt.Fprintf(log, " ---> %s\n", short(meta.DiffID))

		case Run:
			layerKey := hashStr(cacheKey + "\n" + inst.Raw)
			meta, hit, err := b.cachedLayer(layerKey, opts.NoCache)
			if err != nil {
				return out, err
			}
			if hit {
				_, _ = fmt.Fprintf(log, " ---> Using cache\n")
			} else {
				meta, err = b.layerFromRun(layers, cfg, inst.Args[0], layerKey, log)
				if err != nil {
					return out, err
				}
			}
			layers = append(layers, meta)
			cfg.RootFS.DiffIDs = append(cfg.RootFS.DiffIDs, meta.DiffID)
			cacheKey = layerKey
			_, _ = fmt.Fprintf(log, " ---> %s\n", short(meta.DiffID))
		}
	}

	cfg.Created = time.Now().UTC().Format(time.RFC3339)
	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		return out, err
	}
	cfgSum := sha256.Sum256(cfgBytes)
	cfgDigest := "sha256:" + hex.EncodeToString(cfgSum[:])
	cfgHex := hex.EncodeToString(cfgSum[:])
	if err := state.WriteFileAtomic(b.Store.Layout.BlobPath(cfgHex), cfgBytes, 0o644); err != nil {
		return out, err
	}

	var total int64
	for _, l := range layers {
		total += l.Size
	}
	rec := image.Record{
		ID:           cfgDigest,
		Repo:         ref.Domain + "/" + ref.Path,
		Tag:          ref.Tag,
		ConfigDigest: cfgDigest,
		Created:      cfg.Created,
		Size:         total,
		Layers:       layers,
		Config:       cfg,
		PulledAt:     time.Now().UTC(),
	}
	if err := b.Store.SaveRecord(rec); err != nil {
		return out, err
	}
	_, _ = fmt.Fprintf(log, "Successfully built %s\n", short(cfgDigest))
	_, _ = fmt.Fprintf(log, "Successfully tagged %s:%s\n", rec.Repo, rec.Tag)
	out.Record = rec
	return out, nil
}

func (b *Builder) resolveBase(ctx context.Context, name string, log io.Writer) (image.Record, error) {
	if rec, err := b.Store.Resolve(name); err == nil {
		return rec, nil
	}
	_, _ = fmt.Fprintf(log, "Pulling base %s…\n", name)
	return b.Store.Pull(ctx, image.PullOptions{Ref: name, Stderr: log})
}

func (b *Builder) cachePath(key string) string {
	return filepath.Join(b.Store.Layout.Root, "buildcache", key+".json")
}

func (b *Builder) cachedLayer(key string, noCache bool) (image.LayerMeta, bool, error) {
	if noCache {
		return image.LayerMeta{}, false, nil
	}
	data, err := os.ReadFile(b.cachePath(key))
	if err != nil {
		return image.LayerMeta{}, false, nil
	}
	var meta image.LayerMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return image.LayerMeta{}, false, nil
	}
	diffHex := strings.TrimPrefix(meta.DiffID, "sha256:")
	if _, err := os.Stat(filepath.Join(b.Store.Layout.LayerPath(diffHex), ".petti-unpacked")); err != nil {
		return image.LayerMeta{}, false, nil
	}
	return meta, true, nil
}

func (b *Builder) saveCache(key string, meta image.LayerMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return state.WriteFileAtomic(b.cachePath(key), data, 0o644)
}

func (b *Builder) layerFromCopy(ctxDir string, srcs []string, dest, key string, log io.Writer) (image.LayerMeta, error) {
	tmp, err := os.MkdirTemp("", "petti-copy-*")
	if err != nil {
		return image.LayerMeta{}, err
	}
	defer os.RemoveAll(tmp)

	destPath := filepath.Join(tmp, filepath.Clean("/"+dest))
	if strings.HasSuffix(dest, "/") || len(srcs) > 1 {
		if err := os.MkdirAll(destPath, 0o755); err != nil {
			return image.LayerMeta{}, err
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return image.LayerMeta{}, err
		}
	}
	for _, src := range srcs {
		from := filepath.Join(ctxDir, src)
		info, err := os.Stat(from)
		if err != nil {
			return image.LayerMeta{}, fmt.Errorf("COPY %s: %w", src, err)
		}
		to := destPath
		if info.IsDir() || strings.HasSuffix(dest, "/") || len(srcs) > 1 {
			to = filepath.Join(destPath, filepath.Base(src))
		}
		if err := copyPath(from, to); err != nil {
			return image.LayerMeta{}, err
		}
	}

	// Use content-addressed path after packing (digest from content).
	tmpBlob := filepath.Join(b.Store.Layout.Root, "buildcache", "tmp-"+key+".tar.gz")
	tmpLayer := filepath.Join(b.Store.Layout.Root, "buildcache", "tmp-layer-"+key)
	digest, diffID, size, err := image.PackDirAsGzipLayer(tmp, tmpBlob, tmpLayer)
	if err != nil {
		return image.LayerMeta{}, err
	}
	hexDigest := strings.TrimPrefix(digest, "sha256:")
	diffHex := strings.TrimPrefix(diffID, "sha256:")
	finalBlob := b.Store.Layout.BlobPath(hexDigest)
	finalLayer := b.Store.Layout.LayerPath(diffHex)
	_ = os.MkdirAll(filepath.Dir(finalBlob), 0o755)
	_ = os.Remove(finalBlob)
	if err := os.Rename(tmpBlob, finalBlob); err != nil {
		// cross-device: copy
		if err := copyFile(tmpBlob, finalBlob); err != nil {
			return image.LayerMeta{}, err
		}
		_ = os.Remove(tmpBlob)
	}
	_ = os.RemoveAll(finalLayer)
	if err := os.Rename(tmpLayer, finalLayer); err != nil {
		if err := copyPath(tmpLayer, finalLayer); err != nil {
			return image.LayerMeta{}, err
		}
		_ = os.RemoveAll(tmpLayer)
	}
	meta := image.LayerMeta{Digest: digest, DiffID: diffID, Size: size}
	_ = b.saveCache(key, meta)
	_, _ = fmt.Fprintf(log, " ---> COPY packed %s\n", short(diffID))
	return meta, nil
}

func (b *Builder) layerFromRun(layers []image.LayerMeta, cfg registry.ImageConfig, cmd, key string, log io.Writer) (image.LayerMeta, error) {
	if os.Geteuid() != 0 {
		return image.LayerMeta{}, fmt.Errorf("RUN requires root (namespaces/overlay); try: sudo petti build …")
	}
	work, err := os.MkdirTemp("", "petti-run-*")
	if err != nil {
		return image.LayerMeta{}, err
	}
	defer os.RemoveAll(work)

	var lowers []string
	for _, l := range layers {
		hex := strings.TrimPrefix(l.DiffID, "sha256:")
		lowers = append(lowers, b.Store.Layout.LayerPath(hex))
	}
	ovDir := filepath.Join(work, "rootfs")
	ov, err := rootfs.MountOverlay(ovDir, lowers)
	if err != nil {
		return image.LayerMeta{}, fmt.Errorf("overlay for RUN: %w", err)
	}
	defer func() { _ = ov.Unmount() }()

	wd := cfg.Config.WorkingDir
	if wd == "" {
		wd = "/"
	}
	hostname := "build"
	proc, err := runtime.Start(runtime.Config{
		ID:          "build-" + key[:12],
		Rootfs:      ov.Merged,
		Hostname:    hostname,
		Workdir:     wd,
		Env:         cfg.Config.Env,
		Args:        []string{"/bin/sh", "-c", cmd},
		NetworkMode: "host",
		Seccomp:     "", // default
		Stdout:      log,
		Stderr:      log,
	})
	if err != nil {
		return image.LayerMeta{}, fmt.Errorf("RUN %q: %w", cmd, err)
	}
	if err := proc.Wait(); err != nil {
		return image.LayerMeta{}, fmt.Errorf("RUN %q: %w", cmd, err)
	}

	// Pack upperdir as the new layer.
	tmpBlob := filepath.Join(work, "layer.tar.gz")
	tmpLayer := filepath.Join(work, "unpacked")
	digest, diffID, size, err := image.PackDirAsGzipLayer(ov.Upper, tmpBlob, tmpLayer)
	if err != nil {
		return image.LayerMeta{}, err
	}
	hexDigest := strings.TrimPrefix(digest, "sha256:")
	diffHex := strings.TrimPrefix(diffID, "sha256:")
	finalBlob := b.Store.Layout.BlobPath(hexDigest)
	finalLayer := b.Store.Layout.LayerPath(diffHex)
	_ = os.MkdirAll(filepath.Dir(finalBlob), 0o755)
	if err := copyFile(tmpBlob, finalBlob); err != nil {
		return image.LayerMeta{}, err
	}
	_ = os.RemoveAll(finalLayer)
	if err := copyPath(tmpLayer, finalLayer); err != nil {
		return image.LayerMeta{}, err
	}
	meta := image.LayerMeta{Digest: digest, DiffID: diffID, Size: size}
	_ = b.saveCache(key, meta)
	return meta, nil
}

func parseLocalTag(tag string) (registry.Reference, error) {
	if !strings.Contains(tag, "/") && !strings.Contains(strings.Split(tag, ":")[0], ".") {
		// myapp:1.0 → localhost/myapp:1.0
		tag = "localhost/" + tag
	}
	return registry.ParseReference(tag)
}

func hashStr(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func short(d string) string {
	d = strings.TrimPrefix(d, "sha256:")
	if len(d) > 12 {
		return d[:12]
	}
	return d
}

func upsertEnv(env []string, kv string) []string {
	key := kv
	if i := strings.IndexByte(kv, '='); i >= 0 {
		key = kv[:i]
	}
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	found := false
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			out = append(out, kv)
			found = true
			continue
		}
		out = append(out, e)
	}
	if !found {
		out = append(out, kv)
	}
	return out
}

func fileFingerprint(ctxDir string, srcs []string) string {
	h := sha256.New()
	for _, s := range srcs {
		_ = filepath.Walk(filepath.Join(ctxDir, s), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			fmt.Fprintf(h, "%s %d %d\n", path, info.Size(), info.ModTime().UnixNano())
			return nil
		})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func copyPath(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(src)
		if err != nil {
			return err
		}
		_ = os.MkdirAll(filepath.Dir(dst), 0o755)
		_ = os.Remove(dst)
		return os.Symlink(link, dst)
	}
	if info.IsDir() {
		return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(src, path)
			target := filepath.Join(dst, rel)
			if fi.IsDir() {
				return os.MkdirAll(target, fi.Mode())
			}
			return copyFile(path, target)
		})
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func runtimeGOARCH() string {
	return stdruntime.GOARCH
}
