package azureBlob

import (
	"encoding/base64"
	"fmt"
	"io"

	log "github.com/sirupsen/logrus"

	"github.com/Azure/azure-sdk-for-go/storage"
)

type blockblobWriter struct {
	blockList []storage.Block
	cnt       int
	b         *azureBlob
}

// NewBlockBlobWriter initializes a new io.WriteCloser
// specific for azureBlob
func NewBlockBlobWriter(b *azureBlob) (io.WriteCloser, error) {
	err := b.client.CreateBlockBlob(b.Path(), b.Name())
	if err != nil {
		return nil, err
	}

	return &blockblobWriter{
		b:         b,
		blockList: make([]storage.Block, 0),
		cnt:       0,
	}, nil
}

func (w *blockblobWriter) Write(p []byte) (int, error) {
	nextBlock64 := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%5d", w.cnt)))
	//	log.WithFields(log.Fields{"len(p)": len(p), "w.b.Path()": w.b.Path(), "w.b.Name()": w.b.Name(), "nextBlock64": nextBlock64, "w.cnt": w.cnt}).Debug("azureBlob::blockblobWriter::Write called")

	w.cnt++

	err := w.b.client.PutBlock(w.b.Path(), w.b.Name(), nextBlock64, p)
	if err != nil {
		return 0, err
	}

	w.blockList = append(w.blockList, storage.Block{
		ID:     nextBlock64,
		Status: storage.BlockStatusLatest,
	})

	return len(p), nil
}

func (w *blockblobWriter) Close() error {
	log.WithFields(log.Fields{"w.b.Path()": w.b.Path(), "w.b.Name()": w.b.Name(), "len(w.blockList)": len(w.blockList)}).Debug("azureBlob::blockblobWriter::Close called")
	return w.b.client.PutBlockList(w.b.Path(), w.b.Name(), w.blockList)
}
