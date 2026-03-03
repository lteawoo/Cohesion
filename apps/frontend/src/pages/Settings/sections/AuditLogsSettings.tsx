import { useCallback, useEffect, useMemo, useState, type ChangeEvent } from 'react';
import { App, Button, Card, Empty, Input, Pagination, Select, Space, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  getAuditLog,
  listAuditLogs,
  type AuditLogItem,
  type AuditLogListParams,
  type AuditResult,
} from '@/api/audit';
import SettingSectionHeader from '../components/SettingSectionHeader';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

const DEFAULT_PAGE_SIZE = 20;

interface AuditFilterState {
  from: string;
  to: string;
  user: string;
  action: string;
  spaceId: string;
  result?: AuditResult;
}

const defaultFilterState: AuditFilterState = {
  from: '',
  to: '',
  user: '',
  action: '',
  spaceId: '',
  result: undefined,
};

function toRFC3339(value: string): string | undefined {
  const trimmed = value.trim();
  if (trimmed === '') {
    return undefined;
  }
  const parsed = new Date(trimmed);
  if (Number.isNaN(parsed.getTime())) {
    return undefined;
  }
  return parsed.toISOString();
}

function formatDateTime(value: string): string {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  return parsed.toLocaleString();
}

const resultTagColor: Record<AuditResult, string> = {
  success: 'green',
  partial: 'gold',
  failure: 'red',
  denied: 'volcano',
};

const AuditLogsSettings = () => {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const [filters, setFilters] = useState<AuditFilterState>(defaultFilterState);
  const [appliedFilters, setAppliedFilters] = useState<AuditFilterState>(defaultFilterState);
  const [items, setItems] = useState<AuditLogItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);
  const [loading, setLoading] = useState(false);
  const [selectedLog, setSelectedLog] = useState<AuditLogItem | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const fetchLogs = useCallback(async (targetPage: number, targetPageSize: number, targetFilters: AuditFilterState) => {
    const requestParams: AuditLogListParams = {
      page: targetPage,
      pageSize: targetPageSize,
    };

    const from = toRFC3339(targetFilters.from);
    const to = toRFC3339(targetFilters.to);
    if (from) {
      requestParams.from = from;
    }
    if (to) {
      requestParams.to = to;
    }

    const user = targetFilters.user.trim();
    if (user !== '') {
      requestParams.user = user;
    }
    const action = targetFilters.action.trim();
    if (action !== '') {
      requestParams.action = action;
    }
    const spaceID = Number.parseInt(targetFilters.spaceId.trim(), 10);
    if (!Number.isNaN(spaceID) && spaceID > 0) {
      requestParams.spaceId = spaceID;
    }
    if (targetFilters.result) {
      requestParams.result = targetFilters.result;
    }

    setLoading(true);
    try {
      const response = await listAuditLogs(requestParams);
      setItems(response.items);
      setTotal(response.total);
      if (response.items.length === 0) {
        setSelectedLog(null);
      }
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('auditSettings.loadFailed'));
    } finally {
      setLoading(false);
    }
  }, [message, t]);

  useEffect(() => {
    void fetchLogs(page, pageSize, appliedFilters);
  }, [appliedFilters, fetchLogs, page, pageSize]);

  const resultOptions = useMemo(
    () => ([
      { value: 'success', label: t('auditSettings.resultSuccess') },
      { value: 'partial', label: t('auditSettings.resultPartial') },
      { value: 'failure', label: t('auditSettings.resultFailure') },
      { value: 'denied', label: t('auditSettings.resultDenied') },
    ] as const),
    [t]
  );

  const columns: ColumnsType<AuditLogItem> = useMemo(() => [
    {
      title: t('auditSettings.columnOccurredAt'),
      dataIndex: 'occurredAt',
      key: 'occurredAt',
      width: 180,
      render: (value: string) => formatDateTime(value),
    },
    {
      title: t('auditSettings.columnActor'),
      dataIndex: 'actor',
      key: 'actor',
      width: 120,
      ellipsis: true,
    },
    {
      title: t('auditSettings.columnAction'),
      dataIndex: 'action',
      key: 'action',
      width: 180,
      ellipsis: true,
      render: (value: string) => <Text code>{value}</Text>,
    },
    {
      title: t('auditSettings.columnResult'),
      dataIndex: 'result',
      key: 'result',
      width: 100,
      render: (value: AuditResult) => (
        <Tag color={resultTagColor[value]}>{t(`auditSettings.result.${value}`)}</Tag>
      ),
    },
    {
      title: t('auditSettings.columnSpace'),
      dataIndex: 'spaceId',
      key: 'spaceId',
      width: 90,
      render: (value?: number) => (value ? String(value) : '-'),
    },
    {
      title: t('auditSettings.columnTarget'),
      dataIndex: 'target',
      key: 'target',
      ellipsis: true,
    },
    {
      title: t('auditSettings.columnRequestId'),
      dataIndex: 'requestId',
      key: 'requestId',
      width: 150,
      ellipsis: true,
      render: (value: string) => <Text type="secondary">{value}</Text>,
    },
  ], [t]);

  const handleSearch = () => {
    setPage(1);
    setAppliedFilters(filters);
  };

  const handleReset = () => {
    setFilters(defaultFilterState);
    setAppliedFilters(defaultFilterState);
    setPage(1);
    setPageSize(DEFAULT_PAGE_SIZE);
  };

  const handleSelectLog = async (record: AuditLogItem) => {
    setSelectedLog(record);
    setDetailLoading(true);
    try {
      const detail = await getAuditLog(record.id);
      setSelectedLog(detail);
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('auditSettings.loadDetailFailed'));
    } finally {
      setDetailLoading(false);
    }
  };

  return (
    <Space vertical size="small" className="settings-section">
      <SettingSectionHeader
        title={t('auditSettings.sectionTitle')}
        subtitle={t('auditSettings.sectionSubtitle')}
      />

      <Card size="small">
        <Space size="small" wrap>
          <Input
            type="datetime-local"
            value={filters.from}
            onChange={(event: ChangeEvent<HTMLInputElement>) => setFilters((prev) => ({ ...prev, from: event.target.value }))}
            placeholder={t('auditSettings.fromPlaceholder')}
            aria-label={t('auditSettings.fromPlaceholder')}
          />
          <Input
            type="datetime-local"
            value={filters.to}
            onChange={(event: ChangeEvent<HTMLInputElement>) => setFilters((prev) => ({ ...prev, to: event.target.value }))}
            placeholder={t('auditSettings.toPlaceholder')}
            aria-label={t('auditSettings.toPlaceholder')}
          />
          <Input
            value={filters.user}
            onChange={(event: ChangeEvent<HTMLInputElement>) => setFilters((prev) => ({ ...prev, user: event.target.value }))}
            placeholder={t('auditSettings.userPlaceholder')}
            allowClear
            style={{ width: 160 }}
          />
          <Input
            value={filters.action}
            onChange={(event: ChangeEvent<HTMLInputElement>) => setFilters((prev) => ({ ...prev, action: event.target.value }))}
            placeholder={t('auditSettings.actionPlaceholder')}
            allowClear
            style={{ width: 180 }}
          />
          <Input
            value={filters.spaceId}
            onChange={(event: ChangeEvent<HTMLInputElement>) => setFilters((prev) => ({ ...prev, spaceId: event.target.value }))}
            placeholder={t('auditSettings.spacePlaceholder')}
            allowClear
            style={{ width: 110 }}
          />
          <Select<AuditResult>
            value={filters.result}
            onChange={(value: AuditResult | undefined) => setFilters((prev) => ({ ...prev, result: value }))}
            allowClear
            placeholder={t('auditSettings.resultPlaceholder')}
            options={resultOptions}
            style={{ width: 140 }}
          />
          <Button type="primary" onClick={handleSearch}>
            {t('auditSettings.search')}
          </Button>
          <Button onClick={handleReset}>
            {t('auditSettings.reset')}
          </Button>
        </Space>
      </Card>

      <Card size="small" title={t('auditSettings.listTitle')}>
        <Table<AuditLogItem>
          rowKey="id"
          size="small"
          loading={loading}
          columns={columns}
          dataSource={items}
          pagination={false}
          onRow={(record: AuditLogItem) => ({
            onClick: () => {
              void handleSelectLog(record);
            },
            style: { cursor: 'pointer' },
          })}
          locale={{ emptyText: t('auditSettings.emptyLogs') }}
          scroll={{ x: 980 }}
        />
        <Pagination
          current={page}
          pageSize={pageSize}
          total={total}
          showSizeChanger
          pageSizeOptions={[10, 20, 50, 100]}
          onChange={(nextPage: number, nextPageSize: number) => {
            setPage(nextPage);
            setPageSize(nextPageSize);
          }}
          showTotal={(value: number) => t('auditSettings.totalCount', { count: value })}
          style={{ marginTop: 12 }}
        />
      </Card>

      <Card size="small" title={t('auditSettings.detailTitle')} loading={detailLoading}>
        {selectedLog ? (
          <Space vertical size="small" className="settings-stack-full">
            <Space size="small" wrap>
              <Text strong>{t('auditSettings.columnOccurredAt')}:</Text>
              <Text>{formatDateTime(selectedLog.occurredAt)}</Text>
            </Space>
            <Space size="small" wrap>
              <Text strong>{t('auditSettings.columnActor')}:</Text>
              <Text>{selectedLog.actor}</Text>
            </Space>
            <Space size="small" wrap>
              <Text strong>{t('auditSettings.columnAction')}:</Text>
              <Text code>{selectedLog.action}</Text>
            </Space>
            <Space size="small" wrap>
              <Text strong>{t('auditSettings.columnResult')}:</Text>
              <Tag color={resultTagColor[selectedLog.result]}>
                {t(`auditSettings.result.${selectedLog.result}`)}
              </Tag>
            </Space>
            <Space size="small" wrap>
              <Text strong>{t('auditSettings.columnTarget')}:</Text>
              <Text>{selectedLog.target}</Text>
            </Space>
            <Space size="small" wrap>
              <Text strong>{t('auditSettings.columnRequestId')}:</Text>
              <Text type="secondary">{selectedLog.requestId}</Text>
            </Space>
            <Text strong>{t('auditSettings.metadataTitle')}</Text>
            <pre style={{ margin: 0, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
              {JSON.stringify(selectedLog.metadata, null, 2)}
            </pre>
          </Space>
        ) : (
          <Empty description={t('auditSettings.selectRowHint')} />
        )}
      </Card>
    </Space>
  );
};

export default AuditLogsSettings;
