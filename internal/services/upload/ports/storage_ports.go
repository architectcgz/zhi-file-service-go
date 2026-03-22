package ports

import pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"

type (
	BucketResolver     = pkgstorage.BucketResolver
	ObjectReader       = pkgstorage.ObjectReader
	InlineObjectWriter = pkgstorage.InlineObjectWriter
	MultipartManager   = pkgstorage.MultipartManager
	PresignManager     = pkgstorage.PresignManager
)
