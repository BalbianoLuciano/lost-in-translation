// SPDX-License-Identifier: MIT
pragma solidity 0.8.30;

import {Strings} from "@openzeppelin/contracts/utils/Strings.sol";

/// @title Pilar — la geometría de la obra, portada a enteros.
/// @notice El mismo dibujo que hace la app en `apps/web/src/lib/pilar.ts`, pero
///         sin punto flotante, porque la EVM no tiene.
///
/// @dev La traducción es toda la gracia de este archivo, así que va explicada.
///
///      La app trabaja en coordenadas normalizadas: `x` de 0 a 1 sobre el ancho
///      de la pieza, e `y` en unidades de amplitud de junta (negativo sube).
///      Acá esas dos escalas pasan a **milésimos**: `x` va de 0 a 1000 sobre el
///      ancho, e `y` de -2000 a 2000 sobre la amplitud. Con milésimos alcanza y
///      sobra: el dibujo final se mide en unidades de viewBox, y un error de
///      una milésima de ancho es medio píxel en una imagen de 1000 de ancho.
///
///      La curva `Q` no se aproxima ni se subdivide: se pasa tal cual al SVG,
///      que sabe dibujar Bézier cuadráticas. Lo único que hay que llevar a
///      enteros son los tres puntos —arranque, control y final—, y eso es una
///      multiplicación y una división por 1000. El renderer pone los decimales.
///
///      Las divisiones enteras truncan hacia cero. Da igual: el error es de
///      menos de una unidad de viewBox sobre 1260, y es determinista, que es lo
///      único que este dibujo tiene que prometer.
library Pilar {
    // -------------------------------------------------------------- la escala

    /// @dev El ancho de la pieza, en unidades de viewBox. Es 1000 a propósito:
    ///      así la `x` de un perfil —que viene en milésimos de ancho— es
    ///      directamente la coordenada, y la conversión se lee pero no miente.
    int256 internal constant ANCHO = 1000;

    /// @dev El alto de una pieza. En la app sale de cuántos ejercicios tiene el
    ///      tema; acá es fijo, porque el contrato no sabe —ni tiene por qué
    ///      saber— cuánto contenido tiene un tema hoy. Lo que la distinción
    ///      promete es la forma de la pieza, no el tamaño del temario.
    int256 internal constant ALTO = 320;

    /// @dev Aire alrededor del dibujo. Una junta que sube se va hasta una
    ///      amplitud por encima de su línea base, así que sin este margen la
    ///      espiga de la primera pieza quedaría cortada por el borde.
    int256 internal constant AIRE = 130;

    /// @dev La banda de abajo, donde va el rótulo.
    int256 internal constant BANDA = 170;

    /// @dev La amplitud de la junta: 28% del alto de la pieza más chica, igual
    ///      que en la app. Allá el mínimo se calcula entre las dos piezas
    ///      vecinas y se corta en 34px; acá todas las piezas miden lo mismo, así
    ///      que es una constante, y el techo (157 unidades) nunca se alcanza.
    int256 internal constant AMPLITUD = (ALTO * 28) / 100;

    /// @dev Cuántos perfiles de junta hay. El catálogo se valida contra esto.
    uint8 internal constant JUNTAS = 10;

    // ------------------------------------------------------------- la paleta

    /// @dev Los cuatro colores de `design.md` §4. El token se mira en una
    ///      billetera, sin CSS y sin tema claro/oscuro, así que el fondo va
    ///      pintado: es la tinta, y la pieza es hormigón a plena luz.
    string internal constant TINTA = "#15130f";
    string internal constant HORMIGON_LUZ = "#c6c2b8";
    string internal constant HORMIGON = "#a8a49b";
    string internal constant HORMIGON_SOMBRA = "#55524c";

    // ------------------------------------------------------------ los perfiles

    /// @dev Un tramo del perfil. `curva` distingue la `Q` de la `L`; cuando es
    ///      recta, `cx` y `cy` no se miran.
    struct Tramo {
        bool curva;
        int256 cx;
        int256 cy;
        int256 x;
        int256 y;
    }

    /// @dev Un perfil de junta. Arranca siempre en x=0 —los diez lo hacen— a la
    ///      altura `y0`, y de ahí sigue por sus tramos hasta x=1000.
    struct Junta {
        int256 y0;
        Tramo[] tramos;
    }

    /// @notice El perfil número `i`, en el mismo orden que `JOINTS` de `pilar.ts`.
    ///
    /// @dev **Por qué constantes y no storage.** El SDD proponía cargar los diez
    ///      perfiles al desplegar, como el catálogo. Al escribirlo quedó claro
    ///      que no: los perfiles no son contenido, son la misma geometría que la
    ///      app tiene escrita en TypeScript. Guardarlos costaría unos 40 `SSTORE`
    ///      —cerca de 800.000 de gas— para leer después, en cada `tokenURI`, lo
    ///      mismo que el bytecode ya sabe. Y si un perfil saliera mal, la
    ///      reparación es la misma en los dos casos: una v2. El catálogo sí va
    ///      por parámetro, porque el catálogo cambia cuando cambia el contenido;
    ///      estos diez dibujos, no.
    function junta(uint8 i) internal pure returns (Junta memory j) {
        if (i == 0) {
            // recta
            j.tramos = new Tramo[](1);
            j.tramos[0] = _l(1000, 0);
        } else if (i == 1) {
            // diagonal
            j.y0 = -700;
            j.tramos = new Tramo[](1);
            j.tramos[0] = _l(1000, 700);
        } else if (i == 2) {
            // arco hacia abajo
            j.tramos = new Tramo[](1);
            j.tramos[0] = _q(500, 2000, 1000, 0);
        } else if (i == 3) {
            // espiga
            j.tramos = new Tramo[](5);
            j.tramos[0] = _l(340, 0);
            j.tramos[1] = _l(340, -1000);
            j.tramos[2] = _l(660, -1000);
            j.tramos[3] = _l(660, 0);
            j.tramos[4] = _l(1000, 0);
        } else if (i == 4) {
            // arco corrido
            j.tramos = new Tramo[](1);
            j.tramos[0] = _q(280, -2000, 1000, 0);
        } else if (i == 5) {
            // escalón
            j.y0 = -600;
            j.tramos = new Tramo[](3);
            j.tramos[0] = _l(520, -600);
            j.tramos[1] = _l(520, 600);
            j.tramos[2] = _l(1000, 600);
        } else if (i == 6) {
            // arco
            j.tramos = new Tramo[](1);
            j.tramos[0] = _q(500, -2000, 1000, 0);
        } else if (i == 7) {
            // espiga baja
            j.tramos = new Tramo[](5);
            j.tramos[0] = _l(400, 0);
            j.tramos[1] = _l(400, 1000);
            j.tramos[2] = _l(720, 1000);
            j.tramos[3] = _l(720, 0);
            j.tramos[4] = _l(1000, 0);
        } else if (i == 8) {
            // arco corrido al otro lado
            j.tramos = new Tramo[](1);
            j.tramos[0] = _q(740, 2000, 1000, 0);
        } else {
            // diagonal inversa
            j.y0 = 700;
            j.tramos = new Tramo[](1);
            j.tramos[0] = _l(1000, -700);
        }
    }

    // ------------------------------------------------------------- el dibujo

    /// @notice El SVG entero de una obra: `arriba[k]` y `abajo[k]` son los dos
    ///         perfiles de la pieza k, contando de arriba hacia abajo.
    ///
    /// @dev Una distinción de pieza manda un solo par y sale la pieza sola; una
    ///      de obra manda todas las de esa obra y sale el pilar armado. Es el
    ///      mismo código porque es el mismo dibujo: la obra terminada no es otra
    ///      cosa que sus piezas con las juntas cerradas.
    ///
    ///      Las juntas cierran porque la de abajo de una pieza y la de arriba de
    ///      la siguiente son el mismo perfil recorrido en los dos sentidos —eso
    ///      lo garantiza el catálogo—, y porque acá no hay separación: en la app
    ///      el hueco sale del dominio, que baja; una distinción no baja nunca.
    function svg(string memory rotulo, uint8[] memory arriba, uint8[] memory abajo)
        internal
        pure
        returns (string memory)
    {
        int256 ancho = ANCHO + 2 * AIRE;
        // Un pilar de más de 2^255 piezas no existe: son 41 en el catálogo.
        // forge-lint: disable-next-line(unsafe-typecast)
        int256 alto = int256(arriba.length) * ALTO + 2 * AIRE + BANDA;

        string memory cuerpo;
        for (uint256 k; k < arriba.length; ++k) {
            // forge-lint: disable-next-line(unsafe-typecast)
            int256 top = int256(k) * ALTO;
            cuerpo = string.concat(
                cuerpo, '<path d="', pathPieza(arriba[k], abajo[k], top), '"/>', _mechinales(top)
            );
        }

        return string.concat(
            _cabecera(rotulo, ancho, alto), cuerpo, "</g>", _rotulo(rotulo, ancho, alto), "</svg>"
        );
    }

    /// @dev La apertura del SVG y el grupo que agrupa las piezas: el fondo de
    ///      tinta pintado —una billetera no tiene el CSS de la app— y los
    ///      atributos de relleno y trazo puestos una sola vez para todas.
    function _cabecera(string memory rotulo, int256 ancho, int256 alto)
        private
        pure
        returns (string memory)
    {
        return string.concat(
            '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ',
            _num(ancho),
            " ",
            _num(alto),
            '" width="',
            _num(ancho),
            '" height="',
            _num(alto),
            '"><title>',
            rotulo,
            '</title><rect width="100%" height="100%" fill="',
            TINTA,
            '"/><g transform="translate(',
            _num(AIRE),
            " ",
            _num(AIRE),
            ')" fill="',
            HORMIGON_LUZ,
            '" stroke="',
            HORMIGON_SOMBRA,
            '" stroke-width="6" stroke-linejoin="round">'
        );
    }

    /// @dev El nombre, abajo y centrado. En la app el rótulo va al costado, pero
    ///      acá la imagen es cuadrada-ish y se mira sola, sin la columna de
    ///      nombres al lado.
    function _rotulo(string memory rotulo, int256 ancho, int256 alto)
        private
        pure
        returns (string memory)
    {
        return string.concat(
            '<text x="',
            _num(ancho / 2),
            '" y="',
            _num(alto - BANDA / 2),
            '" fill="',
            HORMIGON,
            '" font-family="monospace" font-size="56" text-anchor="middle">',
            rotulo,
            "</text>"
        );
    }

    /// @notice El atributo `d` de una pieza que apoya en la línea base `top`.
    ///
    /// @dev Es el mismo recorrido que arma `buildPilar`: se baja por el borde
    ///      izquierdo hasta el arranque de la junta de arriba, se cruza de
    ///      izquierda a derecha, se baja por el borde derecho hasta el final de
    ///      la junta de abajo, y se vuelve de derecha a izquierda por ella.
    function pathPieza(uint8 juntaArriba, uint8 juntaAbajo, int256 top)
        internal
        pure
        returns (string memory)
    {
        Junta memory a = junta(juntaArriba);
        Junta memory b = junta(juntaAbajo);
        int256 bottom = top + ALTO;

        Tramo memory ultimo = b.tramos[b.tramos.length - 1];

        return string.concat(
            "M",
            _punto(0, top, a.y0),
            _ida(a, top),
            "L",
            _punto(ultimo.x, bottom, ultimo.y),
            _vuelta(b, bottom),
            "Z"
        );
    }

    /// @dev El perfil de izquierda a derecha, sobre la línea base `base`.
    function _ida(Junta memory j, int256 base) private pure returns (string memory d) {
        for (uint256 k; k < j.tramos.length; ++k) {
            Tramo memory t = j.tramos[k];
            d = t.curva
                ? string.concat(d, "Q", _punto(t.cx, base, t.cy), " ", _punto(t.x, base, t.y))
                : string.concat(d, "L", _punto(t.x, base, t.y));
        }
    }

    /// @dev El mismo perfil de derecha a izquierda: así la junta de abajo de una
    ///      pieza es exactamente la de arriba de la siguiente y calzan sin hueco.
    ///      El punto de llegada de cada tramo es el de arranque del anterior, y
    ///      el del primero es el arranque del perfil.
    function _vuelta(Junta memory j, int256 base) private pure returns (string memory d) {
        uint256 n = j.tramos.length;
        for (uint256 k = n; k > 0; --k) {
            Tramo memory t = j.tramos[k - 1];
            int256 haciaX = k - 1 == 0 ? int256(0) : j.tramos[k - 2].x;
            int256 haciaY = k - 1 == 0 ? j.y0 : j.tramos[k - 2].y;
            d = t.curva
                ? string.concat(d, "Q", _punto(t.cx, base, t.cy), " ", _punto(haciaX, base, haciaY))
                : string.concat(d, "L", _punto(haciaX, base, haciaY));
        }
    }

    /// @dev Los mechinales: los agujeros que deja el encofrado. En la app sólo
    ///      aparecen en piezas grandes; acá todas lo son.
    function _mechinales(int256 top) private pure returns (string memory c) {
        int256 cy = top + (ALTO * 62) / 100;
        int16[4] memory xs = [int16(220), 420, 620, 820];
        for (uint256 k; k < 4; ++k) {
            c = string.concat(
                c,
                '<circle cx="',
                _num((int256(xs[k]) * ANCHO) / 1000),
                '" cy="',
                _num(cy),
                '" r="10" fill="',
                HORMIGON_SOMBRA,
                '" stroke="none"/>'
            );
        }
    }

    /// @dev Un punto del SVG: `x` viene en milésimos de ancho y `dy`, en
    ///      milésimos de amplitud sobre la línea base. Acá es donde las dos
    ///      escalas normalizadas de `pilar.ts` se vuelven coordenadas.
    function _punto(int256 x, int256 base, int256 dy) private pure returns (string memory) {
        return string.concat(_num((x * ANCHO) / 1000), " ", _num(base + (dy * AMPLITUD) / 1000));
    }

    function _num(int256 v) private pure returns (string memory) {
        return Strings.toStringSigned(v);
    }

    function _l(int256 x, int256 y) private pure returns (Tramo memory) {
        return Tramo({curva: false, cx: 0, cy: 0, x: x, y: y});
    }

    function _q(int256 cx, int256 cy, int256 x, int256 y) private pure returns (Tramo memory) {
        return Tramo({curva: true, cx: cx, cy: cy, x: x, y: y});
    }
}
