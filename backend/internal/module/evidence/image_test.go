package evidence

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestProcessImageReencodesSupportedImages(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 12, 8))
	img.Set(1, 1, color.RGBA{R: 120, G: 40, B: 10, A: 255})
	for _, tc := range []struct {
		name     string
		encode   func(*bytes.Buffer) error
		wantMIME string
	}{
		{name: "jpeg", encode: func(out *bytes.Buffer) error { return jpeg.Encode(out, img, nil) }, wantMIME: "image/jpeg"},
		{name: "png", encode: func(out *bytes.Buffer) error { return png.Encode(out, img) }, wantMIME: "image/png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var input bytes.Buffer
			if err := tc.encode(&input); err != nil {
				t.Fatal(err)
			}
			processed, err := ProcessImage(bytes.NewReader(input.Bytes()))
			if err != nil {
				t.Fatalf("process: %v", err)
			}
			if processed.MIME != tc.wantMIME || processed.Width != 12 || processed.Height != 8 || len(processed.Bytes) == 0 {
				t.Fatalf("unexpected processed image: %#v", processed)
			}
		})
	}
}

func TestProcessImageRejectsUnsupportedAndOversizedInput(t *testing.T) {
	if _, err := ProcessImage(bytes.NewBufferString("<svg/>")); !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("expected unsupported format, got %v", err)
	}
	if _, err := ProcessImage(bytes.NewReader(make([]byte, MaxFileBytes+1))); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("expected file too large, got %v", err)
	}
}

func TestProcessImageRejectsOversizedDimensionsBeforeFullDecode(t *testing.T) {
	var input bytes.Buffer
	if err := png.Encode(&input, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	raw := input.Bytes()
	binary.BigEndian.PutUint32(raw[16:20], uint32(MaxDimension+1))
	binary.BigEndian.PutUint32(raw[29:33], crc32.ChecksumIEEE(raw[12:29]))

	if _, err := ProcessImage(bytes.NewReader(raw)); !errors.Is(err, ErrInvalidDimensions) {
		t.Fatalf("expected invalid dimensions before pixel decode, got %v", err)
	}
}

func TestProcessImageAcceptsWebPAndRejectsQRCode(t *testing.T) {
	webpInput, err := base64.StdEncoding.DecodeString(testWebPBase64)
	if err != nil {
		t.Fatal(err)
	}
	processed, err := ProcessImage(bytes.NewReader(webpInput))
	if err != nil {
		t.Fatalf("process WebP: %v", err)
	}
	if processed.MIME != "image/png" || processed.Width == 0 || processed.Height == 0 {
		t.Fatalf("unexpected WebP output: %#v", processed)
	}

	qrInput, err := base64.StdEncoding.DecodeString(testQRCodePNGBase64)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ProcessImage(bytes.NewReader(qrInput)); !errors.Is(err, ErrQRCodeDetected) {
		t.Fatalf("expected QR rejection, got %v", err)
	}
}

func TestMemoryObjectStoreCopiesBytes(t *testing.T) {
	store := NewMemoryObjectStore()
	body := []byte("private")
	if err := store.Put(t.Context(), "one", "image/png", body); err != nil {
		t.Fatal(err)
	}
	body[0] = 'X'
	object, err := store.Get(t.Context(), "one")
	if err != nil {
		t.Fatal(err)
	}
	defer object.Body.Close()
	got := new(bytes.Buffer)
	_, _ = got.ReadFrom(object.Body)
	if got.String() != "private" {
		t.Fatalf("unexpected body %q", got.String())
	}
	if err := store.Delete(t.Context(), "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(t.Context(), "one"); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("expected missing object, got %v", err)
	}
}

const testWebPBase64 = "UklGRrIBAABXRUJQVlA4TKUBAAAvSsAYAA8w//M///MfeJAkbXvaSG7m8Q3GfYSBJekwQztm/IcZlgwnmWImn2BK7aFmBtnVir6q//8VOkFE/xm4baTIu8c48ArEo6+B3zFKYln3pqClSCKX0begFTAXFOLXHSyF8cCNcZEG4OywuA4KVVfJCiArU7GAgJI8+lJP/OKMT/fBAjevg1cYB7YVkFuWga2lyPi5I0HFy5YTpWIHg0RZpkniRVW9odHAKOwosWuOGdxIyn2OvaCDvhg/we6TwadPBPbqBV58MsLmMJ8yZnOWk8SRz4N+QoyPL+MnamzMvcE1rHNEr91F9GKZPVUcS9w7PhhH36suB9qPeYb/oLk6cuTiJ0wOK3m5h1cKjW6EVZCYMK7dxcKCBdgP9HkKr9gkAO2P8GKZGWVdIAatQa+1IDpt6qyorVwdy01xdW8Jkfk6xjEXmVQQ+HQdFr6OKhIN34dXWq0+0qr6EJSCeeVLH9+gvGTLyqM65PQ44ihzlTXxQKjKbAvshXgir7Lil9w4L2bvMycmjQcqXaMCO6BlY28i+FOLzbfI1vEqxAhotocAAA=="

const testQRCodePNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAMQAAADEAQAAAADo3bPGAAAAJHRFWHRTb2Z0d2FyZQBRUi1QTkcgaHR0cDovL3FyLnN3dGNoLmNvbS9nj329AAAE8UlEQVR4AWP4jwuMygwGGYb//xkYGPj/f/j///9/BgYGBgYGfgYGBv7//6krw/D/P/////z8///////hw////xn4//P//8///z91ZRj+/+dn4Of/wMD///8H/v/////n/8/PwM///z91ZRj+/+dn4OdnYPj//z/D////////z8/PwM///z91ZRj+/+dn4Of/z/D//wf+///////wn5+fgZ///3/qyjD8/8///z8//3/+/wz///////8DPz/////8//9TV4bh/38GBgZ+fn5+fn5+fn5+fn5+fgYGBv7//6krw/D/////////Z+Bn4P/////////5///////////UlWH4/5+BgZ+B4f8HBv7////////hP8OHDx/+//9PXRmG///5GT785//wgYH//////////8DAwPCB//9/6sow/P//n+E/w39+Bob//////////3/+//wf+P//p64Mw/////9/+MD/n4Hh//////////+B/wPDB/7//6krw/D/Pz8/Az/DBwaG//////////+H//8/MPD//09dGYb///n/M3xg+M/A8IGBgYHh////DAz8/Pz//1NXhuH//w8M/PwM/xk+MPz//4GB/8MHhg8M/P///6euDMP//x8YPnzg52f4wP//AwMDA/+H//wfGP7//09dGYb/////5+f/wMDAwP+BgYGBgYHh/4f//P///6euDMP//x8+/P//n4HhA/+HDwz8DB8YPjDwf+D//5+6Mgz///Pz8/Pzf2Bg4P//gf8//39+Bob/H/j//6euDMP///z8/z98+MDwgf//////+f/z8zPwf+D//5+6Mgz///P//8//gYHhw/////9/+P+f4f//Dwz8//9TV4bh/3/+Dx/+8zMw/Of///8/w/8PDP///+fn//+fujIM///z///AwP+fgf////8f+P9/+MDA////////qSvD8P//f37+Dwz/P/D/Z2Bg+P///3+G//8//P//n7oyDP//8/P/5+fn/8//n4GB////D/wfPnxg4P//n7oyDP/////w4T/D///8////52dgYGBgYOBn4P//n7oyDP///2f4wP+Bn+H//////39gYODn//Cfgf//f+rKMPz///8/w4f//Az8/z/8//+fgYGB/z///////1NXhuH///////N/4P/w////////f2Bg+P//P/////+pK8Pw/z8DP/8HBv4P/P//f/j//////x/+MzDw//9PXRmG///5/3/gZ+D/wPD/PwMDAwMDAz8/P/////+pK8Pw/z///w8fPvz/wPD/P8P//x/+f2DgZ2Dg//+fujIM///z/+fn5+f/z/D/////////52dgYOD///8/dWUY/v//////fwb+/x/4////z8DA8J//PwP/////qSvD8P8/AwMDP8P/////////n4GBgYGfn4Hh////1JVh+P+f//9//g/8//////////8PHxj4//N/4P//n7oyDP//8zPw8zPw/////8P///8//P/PwMDP8P//f+rKMPz/z8/Az8////////////////8H/g8f/v///5+6Mgz///Mz8PMz/P////8H/v///////4GB4cP///+pK8Pw/z/////8/Pz/////wP///wd+BoYPHz78//+fujIM//8zMDDwM/z/////f4b/DAwfGBg+fPj///9/6sow/McFRmUGgwwA81jI0ePTTjcAAAAASUVORK5CYII="
