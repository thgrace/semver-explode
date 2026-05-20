package purl

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type PURL struct {
	Type       string
	Namespace  string
	Name       string
	Version    string
	Qualifiers map[string]string
	Subpath    string
}

var typeRe = regexp.MustCompile(`^[a-z][a-z0-9.\-]*$`)
var qualKeyRe = regexp.MustCompile(`^[a-z][a-z0-9._\-]*$`)

func Parse(s string) (PURL, error) {
	// Step 1: require "pkg:" prefix, strip it, then strip leading slashes.
	if !strings.HasPrefix(s, "pkg:") {
		return PURL{}, fmt.Errorf("purl: missing \"pkg:\" prefix")
	}
	s = strings.TrimPrefix(s, "pkg:")
	s = strings.TrimLeft(s, "/")

	// Step 2: strip subpath (#), qualifiers (?), version (last @).
	var subpath, qualRaw, version string
	var hasSubpath bool

	if idx := strings.IndexByte(s, '#'); idx >= 0 {
		hasSubpath = true
		subpath = s[idx+1:]
		s = s[:idx]
	}
	if idx := strings.IndexByte(s, '?'); idx >= 0 {
		qualRaw = s[idx+1:]
		s = s[:idx]
	}
	// last literal '@' separates version; encoded '@' (%40) is part of name/namespace.
	if idx := strings.LastIndexByte(s, '@'); idx >= 0 {
		version = s[idx+1:]
		s = s[:idx]
	}

	// Step 3: split type from package path on first '/'.
	slash := strings.IndexByte(s, '/')
	if slash < 0 {
		return PURL{}, fmt.Errorf("purl: missing '/' between type and name")
	}
	rawType := s[:slash]
	pkgPath := s[slash+1:]
	if rawType == "" {
		return PURL{}, fmt.Errorf("purl: empty type")
	}
	if pkgPath == "" {
		return PURL{}, fmt.Errorf("purl: empty package path")
	}

	// Step 4: lowercase and validate type.
	typ := strings.ToLower(rawType)
	if !typeRe.MatchString(typ) {
		return PURL{}, fmt.Errorf("purl: invalid type %q", rawType)
	}

	// Step 5: trim leading/trailing slashes, split into namespace + name.
	pkgPath = strings.Trim(pkgPath, "/")
	segments := strings.Split(pkgPath, "/")
	name := segments[len(segments)-1]
	var nsParts []string
	if len(segments) > 1 {
		nsParts = segments[:len(segments)-1]
	}

	// Step 6: percent-decode segments; reject empty decoded namespace segments.
	decodedNS := make([]string, 0, len(nsParts))
	for _, seg := range nsParts {
		dec, err := url.PathUnescape(seg)
		if err != nil {
			return PURL{}, fmt.Errorf("purl: malformed percent-encoding in namespace: %w", err)
		}
		if dec == "" {
			return PURL{}, fmt.Errorf("purl: empty namespace segment after decoding")
		}
		if strings.Contains(dec, "/") {
			return PURL{}, fmt.Errorf("purl: namespace segment contains '/' after decoding")
		}
		decodedNS = append(decodedNS, dec)
	}
	namespace := strings.Join(decodedNS, "/")

	decodedName, err := url.PathUnescape(name)
	if err != nil {
		return PURL{}, fmt.Errorf("purl: malformed percent-encoding in name: %w", err)
	}
	if decodedName == "" {
		return PURL{}, fmt.Errorf("purl: empty name")
	}
	if strings.Contains(decodedName, "/") {
		return PURL{}, fmt.Errorf("purl: name contains '/' after decoding")
	}

	var decodedVersion string
	if version != "" {
		decodedVersion, err = url.PathUnescape(version)
		if err != nil {
			return PURL{}, fmt.Errorf("purl: malformed percent-encoding in version: %w", err)
		}
	}

	var decodedSubpath string
	if hasSubpath {
		decodedSubpath, err = decodeSubpath(subpath)
		if err != nil {
			return PURL{}, err
		}
	}

	// Step 7: parse qualifiers.
	quals := map[string]string{}
	if qualRaw != "" {
		for _, pair := range strings.Split(qualRaw, "&") {
			if pair == "" {
				continue
			}
			eq := strings.IndexByte(pair, '=')
			if eq < 0 {
				return PURL{}, fmt.Errorf("purl: malformed qualifier %q", pair)
			}
			k := pair[:eq]
			v := pair[eq+1:]
			if !qualKeyRe.MatchString(k) {
				return PURL{}, fmt.Errorf("purl: invalid qualifier key %q", k)
			}
			if v == "" {
				continue
			}
			decV, err := url.PathUnescape(v)
			if err != nil {
				return PURL{}, fmt.Errorf("purl: malformed percent-encoding in qualifier value: %w", err)
			}
			if _, dup := quals[k]; dup {
				return PURL{}, fmt.Errorf("purl: duplicate qualifier key %q", k)
			}
			quals[k] = decV
		}
	}

	return PURL{
		Type:       typ,
		Namespace:  namespace,
		Name:       decodedName,
		Version:    decodedVersion,
		Qualifiers: quals,
		Subpath:    decodedSubpath,
	}, nil
}

func (p PURL) Ecosystem() string {
	switch p.Type {
	case "npm", "pypi":
		return p.Type
	default:
		return ""
	}
}

func (p PURL) PackageName() (string, error) {
	switch p.Type {
	case "npm":
		return npmPackageName(p)
	case "pypi":
		return pypiPackageName(p)
	default:
		return "", fmt.Errorf("purl: unsupported type %q", p.Type)
	}
}

var pypiNormRe = regexp.MustCompile(`[-_.]+`)

func npmPackageName(p PURL) (string, error) {
	ns := strings.ToLower(p.Namespace)
	name := strings.ToLower(p.Name)
	if ns == "" {
		return name, nil
	}
	if !strings.HasPrefix(ns, "@") {
		return "", fmt.Errorf("purl: npm namespace must be a scope beginning with \"@\", got %q", p.Namespace)
	}
	return ns + "/" + name, nil
}

func pypiPackageName(p PURL) (string, error) {
	if p.Namespace != "" {
		return "", fmt.Errorf("purl: pypi purl must not have a namespace, got %q", p.Namespace)
	}
	normalized := strings.ToLower(p.Name)
	normalized = pypiNormRe.ReplaceAllString(normalized, "-")
	return normalized, nil
}

func decodeSubpath(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("purl: empty subpath segment")
	}

	segments := strings.Split(raw, "/")
	decoded := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg == "" {
			return "", fmt.Errorf("purl: empty subpath segment")
		}
		dec, err := url.PathUnescape(seg)
		if err != nil {
			return "", fmt.Errorf("purl: malformed percent-encoding in subpath: %w", err)
		}
		if dec == "." || dec == ".." {
			return "", fmt.Errorf("purl: invalid subpath segment %q", dec)
		}
		if strings.Contains(dec, "/") {
			return "", fmt.Errorf("purl: subpath segment contains '/' after decoding")
		}
		decoded = append(decoded, dec)
	}
	return strings.Join(decoded, "/"), nil
}
