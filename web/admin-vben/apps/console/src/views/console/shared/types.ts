export interface ConsoleColumn {
  title: string;
  dataIndex: string;
  key: string;
  width?: number;
}

export interface ConsoleSearchField {
  key: string;
  label: string;
  type?: 'cascader' | 'input' | 'select';
  haschange?: boolean;
  onChange?: (value: any) => void;
  options?: Array<{ label: string; value: number | string }>;
  loadData?: (selectedOptions: any[]) => void;
}

export interface ConsoleFormField {
  key: string;
  label: string;
  type?:
    | 'cascader'
    | 'input'
    | 'number'
    | 'radio'
    | 'select'
    | 'textarea'
    | 'uploadImg';
  required?: boolean;
  options?: Array<{ label: string; value: number | string }>;
  /** 是否不显示 */
  isNotShow?: boolean;
  /** 是否有值变化事件 */
  haschange?: boolean;
  /** 值变化回调 */
  onChange?: (value: any) => void;
  /** 加载数据（级联选择器用） */
  loadData?: (selectedOptions: any[]) => void;
  /** 存储驱动类型（上传组件用） */
  storageType?: 'cos' | 'oss' | 's3';
  /** 文件路径前缀（上传组件用） */
  prefix?: string;
  /** 最大文件大小，单位MB（上传组件用） */
  maxSize?: number;
  /** 最大文件数量（上传组件用） */
  maxCount?: number;
  /** 上传按钮文字（上传组件用） */
  uploadText?: string;
}
