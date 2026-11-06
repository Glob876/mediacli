export interface ActiveReport { count: number; lang: string }
export interface TaskSnapshot {
  id: number; title: string; source: string; target: string;
  status: 'queued' | 'running' | 'done' | 'failed' | 'cancelled' | 'interrupted';
  stage: string; progress: number; exit_code: number; finalized: boolean;
  started_at: string; finished_at?: string; log_tail?: string[];
  group_id?: string; chain_index?: number; chain_total?: number;
}
export interface Operation { type: string; config: Record<string, unknown> }
export interface Pipeline {
  inputs: string[]; operations: Operation[];
  output: { path: string; overwrite: boolean; target_megabytes?: number; two_pass?: boolean };
  name?: string; id?: string;
}
