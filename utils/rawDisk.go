package utils

import (
	"archive/tar"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/kairos-io/kairos-sdk/constants"
	"github.com/kairos-io/kairos-sdk/types"
)

// Raw2Azure converts a raw disk to a VHD disk compatible with Azure
// All VHDs on Azure must have a virtual size aligned to 1 MB (1024 × 1024 bytes)
// The Hyper-V virtual hard disk (VHDX) format isn't supported in Azure, only fixed VHD
func Raw2Azure(source string, logger types.KairosLogger) error {
	logger.Logger.Info().Str("source", source).Msg("Converting raw disk to Azure VHD")
	// Copy raw to new image with VHD appended
	// rename file to .vhd
	err := os.Rename(source, fmt.Sprintf("%s.vhd", source))
	if err != nil {
		logger.Logger.Error().Err(err).Str("source", source).Msg("Error renaming raw image to vhd")
		return err
	}
	// Open it
	vhdFile, _ := os.OpenFile(fmt.Sprintf("%s.vhd", source), os.O_APPEND|os.O_WRONLY, constants.FilePerm)
	// Calculate rounded size
	info, err := vhdFile.Stat()
	if err != nil {
		logger.Logger.Error().Err(err).Str("source", source).Msg("Error getting file info")
		return err
	}
	actualSize := info.Size()
	finalSizeBytes := ((actualSize + 1024*1024 - 1) / 1024 * 1024) * 1024 * 1024
	// Don't forget to remove 512 bytes for the header that we are going to add afterwards!
	finalSizeBytes = finalSizeBytes - 512
	// For smaller than 1 MB images, this calculation doesn't work, so we round up to 1 MB
	if finalSizeBytes == 0 {
		finalSizeBytes = 1*1024*1024 - 512
	}
	if actualSize != finalSizeBytes {
		logger.Logger.Info().Int64("actualSize", actualSize).Int64("finalSize", finalSizeBytes).Msg("Resizing image")
		// If you do not seek, you will override the data
		_, err = vhdFile.Seek(0, io.SeekEnd)
		if err != nil {
			logger.Logger.Error().Err(err).Str("source", source).Msg("Error seeking to end")
			return err
		}
		err = vhdFile.Truncate(finalSizeBytes)
		if err != nil {
			logger.Logger.Error().Err(err).Str("source", source).Msg("Error truncating file")
			return err
		}
	}
	// Transform it to VHD
	info, err = vhdFile.Stat() // Stat again to get the new size
	if err != nil {
		logger.Logger.Error().Err(err).Str("source", source).Msg("Error getting file info")
		return err
	}
	size := uint64(info.Size())
	header := newVHDFixed(size)
	err = binary.Write(vhdFile, binary.BigEndian, header)
	if err != nil {
		logger.Logger.Error().Err(err).Str("source", source).Msg("Error writing header")
		return err
	}
	err = vhdFile.Close()
	if err != nil {
		logger.Logger.Error().Err(err).Str("source", source).Msg("Error closing file")
		return err
	}
	return nil
}

// Raw2Gce transforms an image from RAW format into GCE format
// The RAW image file must have a size in an increment of 1 GB. For example, the file must be either 10 GB or 11 GB but not 10.5 GB.
// The disk image filename must be disk.raw.
// The compressed file must be a .tar.gz file that uses gzip compression and the --format=oldgnu option for the tar utility.
func Raw2Gce(source string, kairosFs types.KairosFS, logger types.KairosLogger) error {
	logger.Logger.Info().Msg("Transforming raw image into gce format")
	actImg, err := kairosFs.OpenFile(source, os.O_CREATE|os.O_APPEND|os.O_WRONLY, constants.FilePerm)
	if err != nil {
		logger.Logger.Error().Err(err).Str("file", source).Msg("Error opening file")
		return err
	}
	info, err := actImg.Stat()
	if err != nil {
		logger.Logger.Error().Err(err).Str("file", source).Msg("Error getting file info")
		return err
	}
	actualSize := info.Size()
	finalSizeGB := actualSize/constants.GB + 1
	finalSizeBytes := finalSizeGB * constants.GB
	logger.Logger.Info().Int64("current", actualSize).Int64("final", finalSizeGB).Str("file", source).Msg("Resizing image")
	// REMEMBER TO SEEK!
	_, err = actImg.Seek(0, io.SeekEnd)
	if err != nil {
		logger.Logger.Error().Err(err).Str("file", source).Msg("Error seeking to end")
		return err
	}
	err = actImg.Truncate(finalSizeBytes)
	if err != nil {
		logger.Logger.Error().Err(err).Str("file", source).Msg("Error truncating file")
		return err
	}
	err = actImg.Close()
	if err != nil {
		logger.Logger.Error().Err(err).Str("file", source).Msg("Error closing file")
		return err
	}

	// Tar gz the image

	// Create destination file
	file, err := kairosFs.Create(fmt.Sprintf("%s.tar.gz", source))
	if err != nil {
		logger.Logger.Error().Err(err).Str("destination", fmt.Sprintf("%s.tar.gz", source)).Msg("Error creating destination file")
		return err
	}
	logger.Logger.Info().Str("destination", file.Name()).Msg("Compressing raw image into a tar.gz")

	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			logger.Logger.Error().Err(err).Str("destination", file.Name()).Msg("Error closing destination file")
		}
	}(file)
	// Create gzip writer
	gzipWriter, err := gzip.NewWriterLevel(file, gzip.BestSpeed)
	if err != nil {
		return err
	}
	defer func(gzipWriter *gzip.Writer) {
		err := gzipWriter.Close()
		if err != nil {
			logger.Logger.Error().Err(err).Str("destination", file.Name()).Msg("Error closing gzip writer")
		}
	}(gzipWriter)
	// Create tarwriter pointing to our gzip writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer func(tarWriter *tar.Writer) {
		err = tarWriter.Close()
		if err != nil {
			logger.Logger.Error().Err(err).Str("destination", file.Name()).Msg("Error closing tar writer")
		}
	}(tarWriter)

	// Open disk.raw
	sourceFile, _ := kairosFs.Open(source)
	sourceStat, _ := sourceFile.Stat()
	defer func(sourceFile fs.File) {
		err = sourceFile.Close()
		if err != nil {
			logger.Logger.Error().Err(err).Str("source", source).Msg("Error closing source file")
		}
	}(sourceFile)

	// Add disk.raw file
	header := &tar.Header{
		Name:   sourceStat.Name(),
		Size:   sourceStat.Size(),
		Mode:   int64(sourceStat.Mode()),
		Format: tar.FormatGNU,
	}
	// Write header with all the info
	err = tarWriter.WriteHeader(header)
	if err != nil {
		logger.Logger.Error().Err(err).Str("source", source).Msg("Error writing header")
		return err
	}
	// copy the actual data
	_, err = io.Copy(tarWriter, sourceFile)
	if err != nil {
		logger.Logger.Error().Err(err).Str("source", source).Msg("Error copying data")
		return err
	}
	// Remove full raw image, we already got the compressed one
	err = kairosFs.RemoveAll(source)
	if err != nil {
		logger.Logger.Error().Err(err).Str("source", source).Msg("Error removing full raw image")
		return err
	}
	return nil
}
