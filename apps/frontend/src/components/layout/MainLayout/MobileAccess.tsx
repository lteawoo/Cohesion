import { Modal, QRCode, Select, Space, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { mobileAddresses } from './mobileAddresses';

export default function MobileAccess({ hosts, onClose }: { hosts: string[]; onClose: () => void }) {
  const { t } = useTranslation();
  const [selected, setSelected] = useState('');
  const urls = mobileAddresses(hosts);
  const url = urls.includes(selected) ? selected : urls[0];
  return (
    <Modal open onCancel={onClose} footer={null} title={t('serverStatus.openOnPhone')}>
      {url ? (
        <Space vertical size="middle" style={{ width: '100%' }}>
          <Typography.Text>{t('serverStatus.wifiScan')}</Typography.Text>
          {urls.length > 1 && <Select aria-label={t('serverStatus.lanAddress')} style={{ width: '100%' }}
            value={url} onChange={setSelected} options={urls.map(value => ({ label: value, value }))} />}
          <QRCode value={url} type="svg" size={192} bgColor="#fff" color="#111" />
          <Typography.Text copyable>{url}</Typography.Text>
        </Space>
      ) : <Typography.Text>{t('serverStatus.noLanAddress')}</Typography.Text>}
    </Modal>
  );
}
