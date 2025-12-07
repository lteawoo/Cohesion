import { Modal, } from "antd";
import { useEffect, useState } from "react";
import type { FileInfo } from "../types";
import { HomeFilled } from "@ant-design/icons";
import { RiHardDrive3Fill } from "react-icons/ri";

export default function DirectorySetupModal({ isOpen, onClose } : { isOpen: boolean, onClose: () => void}) {
  const [loading, setLoading] = useState<boolean>(true);
  const [baseDirectories, setBaseDirectories] = useState<FileInfo[]>([]);

  useEffect(() => {
    const fetchBaseDir = async () => {
      try {
        const response = await fetch('/api/browse/base-directories');
        const data = await response.json();
        console.log('Base Directories:', data);

        if (!response.ok) {
          throw new Error(data.message || 'Failed to fetch base directories');
        }

        setBaseDirectories(data);
      } catch (error) {
        console.error('Error fetching base directories:', error);
      } finally {
        setLoading(false);
      }
    }
    fetchBaseDir();
  }, []);

  return (
    <Modal
        title="Space 등록"
        open={isOpen}
        onCancel={onClose}
        width= {{
          xs: '90%',
          md: '70%',
          lg: '60%',
        }}>
      <div style={{ display: 'flex', flexDirection: 'row', gap: '16px' }}>
        {/* user folder and disk */}
        <div style={{ flex: 0.7 }}>
          {baseDirectories.map((dir: FileInfo) => {
            if (dir.name === 'Home') {
              return <div key={dir.path}><HomeFilled /> Home</div>
            } else {
              return <div key={dir.path}><RiHardDrive3Fill /> {dir.name}</div>
            }
          })}
        </div>
        {/* directory list */}
        <div style={{ flex: 1 }}  >
          directory list
        </div>
      </div>
    </Modal>
  );
}