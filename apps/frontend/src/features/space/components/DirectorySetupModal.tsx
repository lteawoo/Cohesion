import { Alert, App, Input, Modal, theme } from "antd";
import FolderTree from "../../browse/components/FolderTree";
import { useEffect, useRef, useState } from "react";
import { useSpaceStore } from "@/stores/spaceStore";
import { useTranslation } from "react-i18next";
import { ApiError } from "@/api/error";
import type { SpaceRootValidationCode, SpaceRootValidationResult } from "../types";

type RootValidationState =
  | { status: 'idle'; message?: string; description?: string }
  | { status: 'validating'; message?: string; description?: string }
  | { status: 'valid'; code: SpaceRootValidationCode; message: string }
  | { status: 'invalid'; code?: SpaceRootValidationCode; message: string; description?: string };

function isSpaceRootValidationCode(value: string | undefined): value is SpaceRootValidationCode {
  return value === 'valid'
    || value === 'not_found'
    || value === 'not_directory'
    || value === 'permission_denied';
}

function buildValidationState(
  t: (key: string) => string,
  result: SpaceRootValidationResult,
): RootValidationState {
  switch (result.code) {
    case 'valid':
      return { status: 'valid', code: result.code, message: t('directorySetup.validation.valid') };
    case 'not_found':
      return { status: 'invalid', code: result.code, message: t('directorySetup.validation.notFound') };
    case 'not_directory':
      return { status: 'invalid', code: result.code, message: t('directorySetup.validation.notDirectory') };
    case 'permission_denied':
      return {
        status: 'invalid',
        code: result.code,
        message: t('directorySetup.validation.permissionDenied'),
        description: t('directorySetup.validation.permissionDeniedHint'),
      };
    default:
      return { status: 'invalid', message: result.message ?? t('directorySetup.validation.invalid') };
  }
}

function buildValidationStateFromApiError(
  t: (key: string) => string,
  error: unknown,
): RootValidationState | null {
  if (!(error instanceof ApiError) || !isSpaceRootValidationCode(error.code)) {
    return null;
  }

  return buildValidationState(t, {
    valid: false,
    code: error.code,
    message: error.message,
  });
}

export default function DirectorySetupModal({
  isOpen,
  onClose
}: {
  isOpen: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const [selectedPath, setSelectedPath] = useState<string>('');
  const [spaceName, setSpaceName] = useState<string>('');
  const [isCreating, setIsCreating] = useState(false);
  const [rootValidation, setRootValidation] = useState<RootValidationState>({ status: 'idle' });
  const createSpace = useSpaceStore((state) => state.createSpace);
  const validateSpaceRoot = useSpaceStore((state) => state.validateSpaceRoot);
  const { token } = theme.useToken();
  const validationRequestIdRef = useRef(0);

  const handleClose = () => {
    if (isCreating) {
      return;
    }
    setSelectedPath('');
    setSpaceName('');
    setRootValidation({ status: 'idle' });
    validationRequestIdRef.current += 1;
    onClose();
  };

  const handleSelect = (path: string) => {
    setSelectedPath(path);
    setRootValidation(path ? { status: 'validating' } : { status: 'idle' });

    // 선택된 폴더 이름을 기본 Space 이름으로 설정
    if (!spaceName) {
      const folderName = path.split('/').pop() || '';
      setSpaceName(folderName);
    }
  };

  const handleOk = async () => {
    // 유효성 검사
    if (!spaceName.trim()) {
      message.error(t('directorySetup.spaceNameRequired'));
      return;
    }

    if (!selectedPath) {
      message.error(t('directorySetup.folderRequired'));
      return;
    }

    if (rootValidation.status === 'validating' || rootValidation.status === 'invalid') {
      message.error(t('directorySetup.validation.invalid'));
      return;
    }

    setIsCreating(true);
    try {
      await createSpace(spaceName.trim(), selectedPath);
      message.success(t('directorySetup.createSuccess'));
      setSelectedPath('');
      setSpaceName('');
      setRootValidation({ status: 'idle' });
      onClose();
    } catch (error) {
      const validationState = buildValidationStateFromApiError(t, error);
      if (validationState) {
        setRootValidation(validationState);
        message.error(validationState.message);
      } else {
        message.error(error instanceof Error ? error.message : t('directorySetup.createFailed'));
      }
    } finally {
      setIsCreating(false);
    }
  };

  useEffect(() => {
    if (!isOpen || !selectedPath) {
      return;
    }

    let active = true;
    const requestId = validationRequestIdRef.current + 1;
    validationRequestIdRef.current = requestId;
    setRootValidation({ status: 'validating' });

    void (async () => {
      try {
        const result = await validateSpaceRoot(selectedPath);
        if (!active || validationRequestIdRef.current !== requestId) {
          return;
        }
        setRootValidation(buildValidationState(t, result));
      } catch (error) {
        if (!active || validationRequestIdRef.current !== requestId) {
          return;
        }
        const validationState = buildValidationStateFromApiError(t, error);
        setRootValidation(validationState ?? {
          status: 'invalid',
          message: error instanceof Error ? error.message : t('directorySetup.validationFailed'),
        });
      }
    })();

    return () => {
      active = false;
    };
  }, [isOpen, selectedPath, t, validateSpaceRoot]);

  const isRootValidationBlocking = rootValidation.status === 'validating' || rootValidation.status === 'invalid';

  return (
    <Modal
      title={t('directorySetup.title')}
      open={isOpen}
      onOk={handleOk}
      onCancel={handleClose}
      width={600}
      okButtonProps={{
        disabled: !selectedPath || !spaceName.trim() || isCreating || isRootValidationBlocking,
        loading: isCreating
      }}
      cancelButtonProps={{ disabled: isCreating }}
      destroyOnHidden={true}
    >
      <div style={{ marginBottom: 16 }}>
        <label style={{ display: 'block', marginBottom: 8, fontWeight: 500 }}>
          {t('directorySetup.spaceNameLabel')} <span style={{ color: token.colorError }}>*</span>
        </label>
        <Input
          placeholder={t('directorySetup.spaceNamePlaceholder')}
          value={spaceName}
          onChange={(e: React.ChangeEvent<HTMLInputElement>) => setSpaceName(e.target.value)}
          maxLength={100}
          disabled={isCreating}
        />
      </div>

      <div style={{ marginBottom: 16 }}>
        <label style={{ display: 'block', marginBottom: 8, fontWeight: 500 }}>
          {t('directorySetup.folderSelectLabel')} <span style={{ color: token.colorError }}>*</span>
        </label>
        <div style={{ fontStyle: 'italic', marginBottom: 8, fontSize: 12 }}>
          {t('directorySetup.selectedFolder')}: {selectedPath || t('directorySetup.none')}
        </div>
        {rootValidation.status !== 'idle' ? (
          <Alert
            style={{ marginBottom: 8 }}
            type={rootValidation.status === 'valid' ? 'success' : rootValidation.status === 'validating' ? 'info' : 'error'}
            message={rootValidation.status === 'validating' ? t('directorySetup.validation.validating') : rootValidation.message}
            description={rootValidation.status === 'invalid' ? rootValidation.description : undefined}
            showIcon={true}
          />
        ) : null}
        <div
          style={{
            height: '40vh',
            overflow: 'auto',
            border: `1px solid ${token.colorBorder}`,
            borderRadius: 4,
            padding: 8,
          }}
        >
          <FolderTree
            onSelect={handleSelect}
            showBaseDirectories={true}
            hidePartialBrowseErrorAlert={true}
          />
        </div>
      </div>
    </Modal>
  );
}
