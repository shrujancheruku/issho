import Svg, { Circle, Path } from 'react-native-svg';

/**
 * The tabs' line icons, drawn from the same paths as the PNGs native tabs
 * use (assets/images/tabIcons/*.svg): two overlapping rings for Together,
 * a folded map for Plans, a person for You.
 */
export function TabIcon({ name, color, size = 26 }: { name: 'together' | 'plans' | 'you'; color: string; size?: number }) {
  return (
    <Svg width={size} height={size} viewBox="0 0 26 26" fill="none" stroke={color} strokeWidth={2} aria-hidden>
      {name === 'together' ? (
        <>
          <Circle cx={9.5} cy={13} r={6} />
          <Circle cx={16.5} cy={13} r={6} />
        </>
      ) : name === 'plans' ? (
        <>
          <Path d="M3 6l6-2 8 3 6-2v15l-6 2-8-3-6 2z" strokeLinejoin="round" />
          <Path d="M9 4v15M17 7v15" />
        </>
      ) : (
        <>
          <Circle cx={13} cy={9} r={4.5} />
          <Path d="M4.5 22c1.2-4.4 4.4-6.5 8.5-6.5s7.3 2.1 8.5 6.5" strokeLinecap="round" />
        </>
      )}
    </Svg>
  );
}
