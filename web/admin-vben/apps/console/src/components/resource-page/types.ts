export interface ConsoleColumn {
  title: string;
  dataIndex: string;
  key: string;
  width?: number;
}

export interface ConsoleOption {
  label: string;
  value: number | string;
}

export interface ConsoleSearchField {
  key: string;
  label: string;
  type?: 'input' | 'select';
  options?: ConsoleOption[];
}

export interface ConsoleFormField {
  key: string;
  label: string;
  type?:
    | 'datetime'
    | 'input'
    | 'number'
    | 'radio'
    | 'select'
    | 'switch'
    | 'textarea'
    | 'uploadImg';
  required?: boolean;
  /** 输入框占位提示 */
  placeholder?: string;
  /** 显示在字段下方的说明 */
  help?: string;
  options?: ConsoleOption[];
  /** 后端存储磁盘（上传组件用） */
  disk?: string;
  /** 后端命名上传策略（上传组件用） */
  purpose?: string;
  /** 接受的文件类型（上传组件用） */
  accept?: string;
  /** 最大文件大小，单位MB（上传组件用） */
  maxSize?: number;
  /** 最大文件数量（上传组件用） */
  maxCount?: number;
  /** 上传按钮文字（上传组件用） */
  uploadText?: string;
}
