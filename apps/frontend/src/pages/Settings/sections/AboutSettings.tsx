import { App, Button, Card, Descriptions, Space, Typography } from 'antd';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import SettingSectionHeader from '../components/SettingSectionHeader';
import { useUpdateCheck } from '@/features/status/hooks/useUpdateCheck';
import { useSystemVersion } from '@/features/status/hooks/useSystemVersion';
import { useSelfUpdate } from '@/features/status/hooks/useSelfUpdate';

const { Text } = Typography;
const REPO_URL = 'https://github.com/lteawoo/Cohesion';

export default function AboutSettings() {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const { updateInfo } = useUpdateCheck();
  const { versionInfo } = useSystemVersion();
  const { status: selfUpdateStatus, isStarting, isUpdating, startUpdate } = useSelfUpdate();

  const currentVersion = useMemo(() => {
    if (updateInfo?.currentVersion) {
      return updateInfo.currentVersion;
    }
    if (versionInfo?.version) {
      return versionInfo.version;
    }
    return 'dev';
  }, [updateInfo?.currentVersion, versionInfo?.version]);

  const latestVersion = updateInfo?.latestVersion ?? '-';
  const canStartUpdate = currentVersion !== 'dev';
  const isForceUpdate = canStartUpdate && updateInfo !== null && !updateInfo.updateAvailable;

  const handleStartUpdate = async () => {
    try {
      await startUpdate(isForceUpdate);
      message.info(isForceUpdate ? t('aboutSettings.reinstallStartHint') : t('aboutSettings.updateStartHint'));
    } catch (error) {
      message.error((error as Error).message || t('aboutSettings.updateFailed'));
    }
  };

  return (
    <Space vertical size="small" className="settings-section">
      <SettingSectionHeader title={t('aboutSettings.sectionTitle')} subtitle={t('aboutSettings.sectionSubtitle')} />

      <Card size="small" title={t('aboutSettings.cohesionCardTitle')}>
        <Space vertical size="small" className="settings-stack-full">
          <Text type="secondary">{t('aboutSettings.cohesionDescription')}</Text>
          <Descriptions
            size="small"
            layout="vertical"
            colon={false}
            column={1}
            labelStyle={{ fontWeight: 600 }}
            items={[
              {
                key: 'github-url',
                label: t('aboutSettings.githubUrlLabel'),
                children: (
                  <a href={REPO_URL} target="_blank" rel="noreferrer">
                    {REPO_URL}
                  </a>
                ),
              },
            ]}
          />
        </Space>
      </Card>

      <Card size="small" title={t('aboutSettings.cardTitle')}>
        <Space vertical size="small" className="settings-stack-full">
          <Descriptions
            size="small"
            layout="vertical"
            colon={false}
            column={1}
            labelStyle={{ fontWeight: 600 }}
            items={[
              {
                key: 'current-version',
                label: t('aboutSettings.currentVersion'),
                children: <Text code>{currentVersion}</Text>,
              },
              {
                key: 'latest-version',
                label: t('aboutSettings.latestVersion'),
                children: <Text code>{latestVersion}</Text>,
              },
            ]}
          />
          {selfUpdateStatus?.message && (
            <Text type={selfUpdateStatus.state === 'failed' ? 'danger' : 'secondary'}>
              {selfUpdateStatus.message}
            </Text>
          )}
          <Button
            type="primary"
            disabled={!canStartUpdate}
            loading={isStarting || isUpdating}
            onClick={handleStartUpdate}
          >
            {isStarting || isUpdating
              ? t('aboutSettings.updateInProgress')
              : (isForceUpdate ? t('aboutSettings.reinstallNow') : t('aboutSettings.updateNow'))}
          </Button>
        </Space>
      </Card>
    </Space>
  );
}
