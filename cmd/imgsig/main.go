package main

import (
	"crypto"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"os/exec"

	_ "image/jpeg"
	_ "image/png"
)

// EXIF keys.
const (
	EXIFComment = "Comment"
	// Perhaps others...
)

type Metadata struct {
	PublicKey []byte
	Signature []byte
}

func uint32SliceToBytes(u []uint32) []byte {
	b := make([]byte, len(u)*4)
	for index, num := range u {
		binary.LittleEndian.PutUint32(b[index*4:index*4+4], num)
	}
	return b
}

func getImageDigest(filename string) ([]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}

	var pix []uint32
	for x := 0; x < img.Bounds().Dx(); x++ {
		for y := 0; y < img.Bounds().Dy(); y++ {
			r, g, b, a := img.At(x, y).RGBA()
			pix = append(pix, r, g, b, a)
		}
	}

	b := sha512.Sum512(uint32SliceToBytes(pix))
	return b[:], nil
}

func writeEXIF(filename, key, value string) error {
	if _, err := exec.Command(
		"exiftool",
		fmt.Sprintf(`-%s="%s"`, key, value),
		filename,
	).Output(); err != nil {
		return fmt.Errorf("exiftool failed: %w", err)
	}
	return nil
}

func getEXIF(filename, key string) ([]byte, error) {
	out, err := exec.Command(
		"exiftool", "-b",
		fmt.Sprintf("-%s", key),
		filename,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("exiftool failed: %w", err)
	}
	if len(out) < 2 {
		return nil, fmt.Errorf("key empty")
	}
	return out[1 : len(out)-1], nil
}

func getRSAKey(filename string) (*rsa.PrivateKey, []byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	// Remember, ioutil is deprecated in Go 1.16!
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, nil, err
	}

	block, rest := pem.Decode(b)
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, err
	}
	return key, rest, nil
}

func sign(filename, rsaPath string) error {
	digest, err := getImageDigest(filename)
	if err != nil {
		return err
	}
	key, _, err := getRSAKey(rsaPath)
	if err != nil {
		return err
	}

	encrypted, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA512, digest)
	if err != nil {
		return err
	}

	publicKey, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return err
	}
	metadata, err := json.Marshal(Metadata{
		PublicKey: publicKey,
		Signature: encrypted,
	})
	if err != nil {
		return err
	}

	return writeEXIF(filename, EXIFComment, string(metadata))
}

func formatFingerprint(f string) string {
	for i := 2; i < len(f); i += 3 {
		f = f[:i] + ":" + f[i:]
	}
	return f
}

func verify(filename string) error {
	comment, err := getEXIF(filename, EXIFComment)
	if err != nil {
		return err
	}

	var metadata Metadata
	if err := json.Unmarshal(comment, &metadata); err != nil {
		return err
	}

	pkix, err := x509.ParsePKIXPublicKey(metadata.PublicKey)
	if err != nil {
		return err
	}
	key, ok := pkix.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("unknown key format %T", key)
	}

	digest, err := getImageDigest(filename)
	if err != nil {
		return err
	}

	if err := rsa.VerifyPKCS1v15(key, crypto.SHA512, digest, metadata.Signature); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	fingerprint := md5.Sum(metadata.PublicKey)
	fmt.Printf("integrity check passed\nRSA public key fingerprint: %s\n",
		formatFingerprint(hex.EncodeToString(fingerprint[:])))

	return nil
}

func main() {
	verifyCmd := flag.NewFlagSet("verify", flag.ExitOnError)
	verifyCmd.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: %s verify path/to/image\n", os.Args[0])
	}

	signCmd := flag.NewFlagSet("sign", flag.ExitOnError)
	rsaKey := signCmd.String("rsa-key", "", "path to an RSA private key")
	signCmd.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `usage: %s sign -author name -rsa-key path/to/key path/to/image
options:`, os.Args[0])
		signCmd.PrintDefaults()
		fmt.Fprintln(flag.CommandLine.Output(), `To generate an RSA key:
  openssl genrsa -out key.pem
  echo "some message" >> key.pem`)
	}

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `usage: %s [command] [arguments]

commands:
  verify    verify the integrity of an image
  sign      sign an image with your private key
`, os.Args[0])
	}

	if len(os.Args) < 2 {
		log.Printf("expected subcommand")
		flag.Usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "verify":
		verifyCmd.Parse(os.Args[2:])
		if len(verifyCmd.Args()) < 1 {
			verifyCmd.Usage()
			os.Exit(1)
		}
		if err := verify(verifyCmd.Arg(0)); err != nil {
			log.Fatal(err)
		}

	case "sign":
		signCmd.Parse(os.Args[2:])
		if len(signCmd.Args()) < 1 || len(*rsaKey) == 0 {
			signCmd.Usage()
			os.Exit(1)
		}
		if err := sign(signCmd.Arg(0), *rsaKey); err != nil {
			log.Fatal(err)
		}

	case "help":
		flag.Usage()
		os.Exit(0)

	default:
		flag.Usage()
		os.Exit(1)
	}
}
