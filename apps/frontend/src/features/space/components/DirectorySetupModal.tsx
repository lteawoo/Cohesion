import { Modal, Form, Input } from "antd";
import { useEffect } from "react";

export default function DirectorySetupModal({ isOpen, onClose } : { isOpen: boolean, onClose: () => void}) {

  useEffect(() => {
    if (!isOpen) {
      // Reset form or perform cleanup
    }
  }, [isOpen]);

  return (
    <Modal
        title="Space 등록"
        open={isOpen}
        onCancel={onClose}
      >
      <Form>
        <Form.Item
          name="spaceName"
          label="Name"
          rules={[{ required: true }]}
        >
          <Input placeholder="Ex) Movie" />
        </Form.Item>

        <Form.Item
          name="spacePath"
          label="Space Path"
          rules={[{ required: true }]}
        >
          <Input placeholder="Ex) 폴더 경로 설정" />
        </Form.Item>
      </Form>
    </Modal>
  );
}