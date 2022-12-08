package binary

import (
	"context"

	"github.com/pkg/errors"
	"github.com/wal-g/tracelog"
	"github.com/wal-g/wal-g/internal"
	"github.com/wal-g/wal-g/internal/databases/mongo/common"
	"github.com/wal-g/wal-g/internal/databases/mongo/models"
)

type RestoreService struct {
	Context      context.Context
	LocalStorage *LocalStorage
	Uploader     *internal.Uploader

	minimalConfigPath string
}

func CreateRestoreService(ctx context.Context, localStorage *LocalStorage, uploader *internal.Uploader,
	minimalConfigPath string) (*RestoreService, error) {
	return &RestoreService{
		Context:           ctx,
		LocalStorage:      localStorage,
		Uploader:          uploader,
		minimalConfigPath: minimalConfigPath,
	}, nil
}

func (restoreService *RestoreService) DoRestore(backupName, restoreMongodVersion string) error {
	sentinel, err := common.DownloadSentinel(restoreService.Uploader.Folder(), backupName)
	if err != nil {
		return err
	}

	err = EnsureCompatibilityToRestoreMongodVersions(restoreMongodVersion, sentinel.MongoMeta.Version)
	if err != nil {
		return err
	}

	err = restoreService.LocalStorage.EnsureMongodFsLockFileIsEmpty()
	if err != nil {
		return err
	}

	err = restoreService.LocalStorage.CleanupMongodDBPath()
	if err != nil {
		return err
	}

	tracelog.InfoLogger.Println("Download backup files to dbPath")
	err = restoreService.downloadFromTarArchives(backupName)
	if err != nil {
		return err
	}

	needFixOplog := NeedFixOplog(restoreMongodVersion)

	if err = restoreService.fixSystemData(sentinel, needFixOplog); err != nil {
		return err
	}

	if needFixOplog {
		if err = restoreService.recoverFromOplogAsStandalone(); err != nil {
			return err
		}
	} else {
		tracelog.InfoLogger.Printf("We are skipping recoverFromOplogAsStandalone because it is disabled")
	}

	return nil
}

func (restoreService *RestoreService) downloadFromTarArchives(backupName string) error {
	downloader := CreateConcurrentDownloader(restoreService.Uploader)
	return downloader.Download(backupName, restoreService.LocalStorage.MongodDBPath)
}

func (restoreService *RestoreService) fixSystemData(sentinel *models.Backup, needFixOplog bool) error {
	mongodProcess, err := StartMongodWithDisableLogicalSessionCacheRefresh(restoreService.minimalConfigPath)
	if err != nil {
		return errors.Wrap(err, "unable to start mongod in special mode")
	}

	mongodService, err := CreateMongodService(restoreService.Context, "wal-g restore", mongodProcess.GetURI())
	if err != nil {
		return errors.Wrap(err, "unable to create mongod service")
	}

	lastWriteTS := sentinel.MongoMeta.BackupLastTS
	err = mongodService.FixSystemDataAfterRestore(lastWriteTS, needFixOplog)
	if err != nil {
		return err
	}

	err = mongodService.Shutdown()
	if err != nil {
		return err
	}

	return mongodProcess.Wait()
}

func (restoreService *RestoreService) recoverFromOplogAsStandalone() error {
	mongodProcess, err := StartMongodWithRecoverFromOplogAsStandalone(restoreService.minimalConfigPath)
	if err != nil {
		return errors.Wrap(err, "unable to start mongod in special mode")
	}

	mongodService, err := CreateMongodService(restoreService.Context, "wal-g restore", mongodProcess.GetURI())
	if err != nil {
		return errors.Wrap(err, "unable to create mongod service")
	}

	err = mongodService.Shutdown()
	if err != nil {
		return err
	}

	return mongodProcess.Wait()
}
