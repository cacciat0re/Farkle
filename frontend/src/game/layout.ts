/**
 * 桌面几何常量：3D 场景布局与离线物理预演必须用同一组数值，
 * 否则骰子落点会和画面里的骰盘对不上。
 */

/** 骰盘内沿半宽 */
export const TRAY = { x: 4.2, z: 2.6 }

/** 骰子边长 */
export const DIE_SIZE = 0.48

/** 骰子六面点数，BoxGeometry 面序 [+x,-x,+y,-y,+z,-z]，对面之和为 7 */
export const FACE_VALUES = [3, 4, 1, 6, 2, 5]

/** 还没开始投掷时，骰子收在桌面下方 */
export const DIE_STASH: [number, number, number] = [0, -3, 0]

/** 锁定骰子陈列位：自己一侧在近端，对手一侧在远端 */
export function lockedSlotX(index: number) {
  return -TRAY.x + 0.7 + index * 0.68
}

export function lockedSlotZ(opposite: boolean) {
  return opposite ? -TRAY.z - 1.4 : TRAY.z + 1.4
}
