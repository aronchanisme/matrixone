// Copyright 2023 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package frontend

import (
	"context"
	"github.com/matrixorigin/matrixone/pkg/backup"
	"github.com/matrixorigin/matrixone/pkg/defines"
	"github.com/matrixorigin/matrixone/pkg/fileservice"
	"github.com/matrixorigin/matrixone/pkg/sql/parsers/tree"
)

func (mce *MysqlCmdExecutor) handleStartBackup(ctx context.Context, sb *tree.BackupStart) error {
	return doBackup(ctx, mce.GetSession(), sb)
}

func doBackup(ctx context.Context, ses *Session, bs *tree.BackupStart) error {
	var (
		err error
	)
	conf := &backup.Config{
		HAkeeper: ses.GetParameterUnit().HAKeeperClient,
	}
	conf.SharedFs, err = fileservice.Get[fileservice.FileService](ses.GetParameterUnit().FileService, defines.SharedFileServiceName)
	if err != nil {
		return err
	}
	return backup.Backup(ctx, bs, conf)
}
