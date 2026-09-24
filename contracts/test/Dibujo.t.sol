// SPDX-License-Identifier: MIT
pragma solidity 0.8.30;

import {VmSafe} from "forge-std/Vm.sol";

import {Base64} from "@openzeppelin/contracts/utils/Base64.sol";
import {IERC721Errors} from "@openzeppelin/contracts/interfaces/draft-IERC6093.sol";

import {Base} from "./Base.t.sol";
import {Distinciones} from "../src/Distinciones.sol";
import {Pilar} from "../src/Pilar.sol";

/// @title El dibujo: que `tokenURI` devuelva una imagen, y que sea la de esta pieza.
///
/// @dev Lo que se prueba acá no es "el SVG es lindo" —eso se mira, y para eso
///      están los archivos de `test/salida`— sino las cuatro propiedades que sí
///      se pueden afirmar: que el URI decodifica, que adentro hay un SVG bien
///      formado, que dos distinciones distintas dan dibujos distintos y que la
///      misma da siempre el mismo.
contract DibujoTest is Base {
    /// @dev Dónde caen los SVG de muestra. Gitignoreado: se rehacen solos.
    string internal constant SALIDA = "test/salida";

    function setUp() public override {
        super.setUp();
        if (!vm.isDir(SALIDA)) vm.createDir(SALIDA, true);
    }

    // ------------------------------------------------------------ el envoltorio

    function test_TokenURIEsUnJSONEnBase64ConUnSVGAdentro() public {
        string memory json = _json(_uri(_acunar(titular, 3)));

        // Que `parseJson` no reviente ya es media prueba: el JSON es válido.
        assertEq(vm.parseJsonString(json, ".name"), "Pieza 3", "el nombre no es el del catalogo");
        assertEq(vm.parseJsonString(json, ".attributes[0].value"), "pieza", "la clase");
        assertEq(vm.parseJsonUint(json, ".attributes[1].value"), 3, "la obra");

        string memory imagen = vm.parseJsonString(json, ".image");
        assertEq(
            vm.indexOf(imagen, "data:image/svg+xml;base64,"),
            0,
            unicode"la imagen tiene que ser un SVG embebido, no un link a un jpeg"
        );
    }

    function test_ElSVGAbreYCierraYTieneElPathDeLaPieza() public {
        string memory svg = _svg(_uri(_acunar(titular, 3)));

        assertEq(vm.indexOf(svg, "<svg"), 0, "el SVG no abre");
        assertTrue(vm.contains(svg, "</svg>"), "el SVG no cierra");
        assertTrue(vm.contains(svg, 'xmlns="http://www.w3.org/2000/svg"'), "sin xmlns no se ve");

        // El `path` de la pieza: arranca con un `M`, tiene juntas y cierra con `Z`.
        assertTrue(vm.contains(svg, '<path d="M'), "no hay path de pieza");
        assertTrue(vm.contains(svg, 'Z"/>'), unicode"el contorno de la pieza no está cerrado");

        // Y es la paleta del proyecto, no la que vino de fábrica.
        assertTrue(vm.contains(svg, "#15130f"), "falta la tinta");
        assertTrue(vm.contains(svg, "#c6c2b8"), unicode"falta el hormigón a la luz");
    }

    /// @dev El mismo dibujo que devuelve el envoltorio, sin las dos capas de base64.
    function test_SvgDeEsElMismoQueVaAdentroDelTokenURI() public {
        uint256 id = _acunar(titular, 11);
        assertEq(distinciones.svgDe(id), _svg(_uri(id)), "el atajo dibuja otra cosa");
    }

    function test_SvgDeTokenInexistenteRevierte() public {
        // El `idDe` va antes del cheatcode: también es una llamada, y se comería
        // el `expectRevert` sin revertir.
        uint256 id = distinciones.idDe(titular, 0);

        vm.expectRevert(abi.encodeWithSelector(IERC721Errors.ERC721NonexistentToken.selector, id));
        distinciones.svgDe(id);
    }

    // -------------------------------------------------------- las dos promesas

    /// @notice Dos piezas distintas dan dos dibujos distintos.
    /// @dev No alcanza con que difiera el rótulo: se compara también el contorno
    ///      solo, que es lo que haría distinta a la pieza aunque se le borrara
    ///      el nombre. Se eligen dos piezas de obras distintas, que es donde el
    ///      catálogo de prueba reparte perfiles distintos.
    function test_DosPiezasDistintasDanDibujosDistintos() public {
        string memory a = distinciones.svgDe(_acunar(titular, 0));
        string memory b = distinciones.svgDe(_acunar(titular, 8));

        assertNotEq(a, b, "dos distinciones distintas dibujan lo mismo");
        assertNotEq(_paths(a), _paths(b), "el contorno es el mismo: cambia solo el cartel");
    }

    /// @notice La misma pieza da siempre el mismo dibujo.
    /// @dev Es la propiedad que hace que el token sea un token: si el dibujo
    ///      dependiera del bloque, del `msg.sender` o de cualquier otra cosa,
    ///      la imagen que guarda una billetera dejaría de coincidir con la que
    ///      devuelve el contrato mañana.
    function test_ElDibujoEsDeterminista() public {
        uint256 id = _acunar(titular, 5);
        string memory primera = distinciones.svgDe(id);

        vm.roll(block.number + 5_000);
        vm.warp(block.timestamp + 365 days);
        vm.prank(makeAddr("otro cualquiera"));
        string memory segunda = distinciones.svgDe(id);

        assertEq(primera, segunda, "el dibujo cambia con el tiempo o con quien mira");
    }

    /// @notice Dos personas con la misma pieza tienen el mismo dibujo.
    /// @dev A propósito: la distinción dice qué se logró, no quién. Lo que
    ///      distingue un token del otro es el dueño, que no se dibuja.
    function test_LaMismaPiezaDeDosTitularesDibujaIgual() public {
        address otro = makeAddr("otra persona");
        assertEq(
            distinciones.svgDe(_acunar(titular, 7)),
            distinciones.svgDe(_acunar(otro, 7)),
            "la misma pieza dibuja distinto segun de quien sea"
        );
    }

    // ------------------------------------------------------------- pieza y obra

    /// @notice Una distinción de obra dibuja más piezas que una de pieza.
    /// @dev La obra terminada es, literalmente, la obra: el pilar entero con las
    ///      juntas cerradas. Una pieza es una sola.
    function test_LaObraDibujaMasPiezasQueUnaPieza() public {
        uint256 unaSola = _cuantosPaths(distinciones.svgDe(_acunar(titular, 0)));
        uint256 elPilar = _cuantosPaths(distinciones.svgDe(_acunar(titular, 34)));

        assertEq(unaSola, 1, "una distincion de pieza dibuja una sola pieza");
        // El catálogo de prueba le da cinco piezas a la obra 0.
        assertEq(elPilar, 5, "la obra 0 tiene cinco piezas en el catalogo de prueba");
        assertGt(elPilar, unaSola, "la obra tendria que dibujar mas que una pieza");
    }

    /// @notice El pilar de la obra es más alto que la pieza sola, y en la misma
    ///         proporción que las piezas que tiene.
    function test_LaObraEsMasAltaQueLaPieza() public {
        uint256 pieza = _altoDelViewBox(distinciones.svgDe(_acunar(titular, 0)));
        uint256 obra = _altoDelViewBox(distinciones.svgDe(_acunar(titular, 34)));

        assertEq(obra - pieza, 4 * uint256(int256(Pilar.ALTO)), "el alto no crece por pieza");
    }

    /// @notice Las juntas del pilar cierran: la de abajo de una pieza es la de
    ///         arriba de la siguiente.
    /// @dev Es lo único que hace que la obra se lea como una obra y no como una
    ///      pila de piedras. Se comprueba sobre el catálogo, que es quien lo
    ///      promete, y no sobre el SVG, donde ya sería tarde para explicar nada.
    function test_LasJuntasDeUnaObraEncadenan() public view {
        for (uint8 obra = 0; obra < 7; ++obra) {
            uint8 anterior = 0; // el pilar arranca con la junta recta
            bool primera = true;
            uint8 ultima = 0;

            for (uint16 i = 0; i < 34; ++i) {
                (, uint8 suObra, uint8 arriba, uint8 abajo,) = distinciones.piezas(i);
                if (suObra != obra) continue;

                assertEq(arriba, anterior, "la junta de arriba no es la de abajo de la anterior");
                anterior = abajo;
                ultima = abajo;
                primera = false;
            }

            assertTrue(primera || ultima == 0, "el pilar no termina con la junta recta");
        }
    }

    /// @notice Los diez perfiles dibujan, y ninguno dibuja lo mismo que otro.
    /// @dev Barre la tabla entera. Si un perfil quedó mal copiado de `pilar.ts`
    ///      —una `Q` escrita como `L`, un signo cambiado— acá se nota, porque
    ///      dos perfiles que tendrían que ser distintos salen iguales o alguno
    ///      sale sin curva donde la tiene.
    function test_LosDiezPerfilesDibujanCosasDistintas() public pure {
        string[] memory vistos = new string[](Pilar.JUNTAS);

        for (uint8 i = 0; i < Pilar.JUNTAS; ++i) {
            string memory d = Pilar.pathPieza(i, 0, 0);
            assertTrue(bytes(d).length > 0, "un perfil no dibujo nada");

            for (uint8 k = 0; k < i; ++k) {
                assertNotEq(d, vistos[k], "dos perfiles distintos dibujan lo mismo");
            }
            vistos[i] = d;
        }

        // Los cuatro arcos son los únicos con Bézier; el resto son rectas.
        assertTrue(vm.contains(Pilar.pathPieza(2, 0, 0), "Q"), "el arco perdio su curva");
        assertTrue(vm.contains(Pilar.pathPieza(6, 0, 0), "Q"), "el arco perdio su curva");
        assertFalse(vm.contains(Pilar.pathPieza(3, 0, 0), "Q"), "la espiga no tiene curvas");
    }

    /// @notice Una obra sin piezas en el catálogo igual dibuja algo.
    /// @dev Las obras 3 a 6 todavía no tienen contenido escrito. Antes que
    ///      revertir en una función que las billeteras llaman solas, se dibuja
    ///      una piedra sola, que es exactamente lo que hay construido.
    function test_UnaObraSinPiezasDibujaUnaPiedraSola() public {
        Distinciones.Pieza[] memory catalogo = new Distinciones.Pieza[](1);
        catalogo[0] = Distinciones.Pieza({
            nombre: unicode"Refugio en la Puna",
            obra: 6,
            juntaArriba: 0,
            juntaAbajo: 0,
            esObra: true
        });

        Distinciones vacia = new Distinciones(duenio, firmante, catalogo);
        distinciones = vacia;

        uint64 deadline = _deadline();
        vacia.mint(titular, 0, deadline, _firmar(PK_FIRMANTE, titular, 0, deadline));

        string memory svg = vacia.svgDe(vacia.idDe(titular, 0));
        assertEq(_cuantosPaths(svg), 1, "una obra sin piezas tendria que dibujar una sola");
    }

    // --------------------------------------------------------------- el catálogo

    function test_UnaJuntaQueNoExisteNoSeDespliega() public {
        Distinciones.Pieza[] memory catalogo = new Distinciones.Pieza[](1);
        catalogo[0] = Distinciones.Pieza({
            nombre: "Rota", obra: 0, juntaArriba: 0, juntaAbajo: Pilar.JUNTAS, esObra: false
        });

        vm.expectRevert(
            abi.encodeWithSelector(Distinciones.JuntaInexistente.selector, 0, Pilar.JUNTAS)
        );
        new Distinciones(duenio, firmante, catalogo);
    }

    /// @notice Un nombre con comillas o con `<` no rompe ni el JSON ni el XML.
    /// @dev El nombre lo pone quien despliega, así que esto no defiende de nadie:
    ///      defiende de una errata en el catálogo que dejaría el `tokenURI` de
    ///      esa distinción ilegible para siempre.
    function test_UnNombreConCaracteresRarosNoRompeElJSON() public {
        Distinciones.Pieza[] memory catalogo = new Distinciones.Pieza[](1);
        catalogo[0] = Distinciones.Pieza({
            nombre: 'Just, "already" & <yet> \\ ever',
            obra: 1,
            juntaArriba: 0,
            juntaAbajo: 0,
            esObra: false
        });

        distinciones = new Distinciones(duenio, firmante, catalogo);

        uint64 deadline = _deadline();
        distinciones.mint(titular, 0, deadline, _firmar(PK_FIRMANTE, titular, 0, deadline));

        string memory json = _json(_uri(distinciones.idDe(titular, 0)));
        assertEq(
            vm.parseJsonString(json, ".name"),
            "Just, &quot;already&quot; &amp; &lt;yet&gt; &#92; ever",
            "el nombre no quedo escapado"
        );
        assertTrue(vm.contains(_svg(_uri(distinciones.idDe(titular, 0))), "&amp;"), "ni en el SVG");
    }

    // ------------------------------------------------------------------- el gas

    /// @notice El costo de cargar el catálogo entero no se escapa sin avisar.
    ///
    /// @dev Es el único gas que este contrato cobra de más respecto de un ERC-721
    ///      pelado, y se paga una sola vez. Los techos están holgados a propósito
    ///      —un 15% arriba de lo medido— para que no haya que tocarlos por una
    ///      versión del compilador, pero lo bastante cerca como para que agregar
    ///      un `SSTORE` por pieza los rompa.
    function test_GasDelDespliegue() public {
        // `forge coverage` compila sin optimizador y con la instrumentación
        // encima: el bytecode es otro y medirlo ahí no dice nada. El número que
        // importa es el de `forge test`, que compila con el mismo perfil que va
        // a la red.
        vm.skip(vm.isContext(VmSafe.ForgeContext.Coverage));

        // `new Distinciones(...)` no sirve para medir: Foundry lo resuelve con un
        // cheatcode y el gas del create no se le cobra al marco del test. Hay que
        // armar el initcode y hacer el CREATE a mano.
        bytes memory initcode = abi.encodePacked(
            type(Distinciones).creationCode, abi.encode(duenio, firmante, _catalogo())
        );

        address nuevo;
        uint256 antes = gasleft();
        assembly {
            nuevo := create(0, add(initcode, 32), mload(initcode))
        }
        uint256 gasDelCreate = antes - gasleft();

        assertTrue(nuevo != address(0), "el CREATE fallo");
        assertEq(Distinciones(nuevo).cantidadPiezas(), CANTIDAD_PIEZAS, "no cargo el catalogo");

        // Lo que se paga de verdad es la transacción entera: el piso de 21.000,
        // los datos —el initcode viaja como calldata— y recién después esto.
        uint256 gasDeLosDatos = 0;
        for (uint256 i; i < initcode.length; ++i) {
            gasDeLosDatos += initcode[i] == 0 ? 4 : 16;
        }
        uint256 total = 21_000 + gasDeLosDatos + gasDelCreate;

        emit log_named_uint("gas del CREATE (constructor + deposito del codigo)", gasDelCreate);
        emit log_named_uint("gas de los datos del initcode", gasDeLosDatos);
        emit log_named_uint("gas de la transaccion de despliegue, completa", total);
        emit log_named_uint("gas por distincion del catalogo", _gasPorDistincion(gasDelCreate));

        // El techo va un 15% arriba de lo medido: suficiente para que no lo
        // rompa un cambio de compilador, y no tanto como para que pase
        // desapercibido un `SSTORE` más por distinción (son ~20.000 por pieza).
        assertLt(total, 6_500_000, unicode"el despliegue se encareció: medilo y decidí a mano");
    }

    /// @notice `tokenURI` es `view`: leerla no cuesta gas de verdad, pero un nodo
    ///         igual la tiene que ejecutar, y hay límites de gas en las consultas.
    /// @dev El del pilar entero es el caso caro: cinco piezas y sus cinco
    ///      contornos, todo concatenado y pasado dos veces por base64.
    function test_GasDeTokenURI() public {
        uint256 pieza = _acunar(titular, 0);
        uint256 obra = _acunar(titular, 34);

        uint256 antes = gasleft();
        distinciones.tokenURI(pieza);
        uint256 gastoPieza = antes - gasleft();

        antes = gasleft();
        distinciones.tokenURI(obra);
        uint256 gastoObra = antes - gasleft();

        emit log_named_uint("gas de tokenURI de una pieza", gastoPieza);
        emit log_named_uint("gas de tokenURI de una obra", gastoObra);

        // El techo por defecto de un `eth_call` anda por los 50.000.000; esto
        // tiene que quedar dos órdenes de magnitud abajo o algo se fue de madre.
        assertLt(gastoObra, 3_000_000, unicode"el dibujo de la obra se puso carísimo");
        assertGt(gastoObra, gastoPieza, "dibujar la obra tendria que costar mas");
    }

    /// @dev Cuánto de ese despliegue es el catálogo: la diferencia entre cargar
    ///      las 41 y cargar una sola, repartida entre las 40 de diferencia. Es
    ///      el número que hay que mirar si algún día se piensa en guardar
    ///      también los perfiles de junta, o el alto de cada pieza.
    function _gasPorDistincion(uint256 gasDeLas41) internal returns (uint256) {
        Distinciones.Pieza[] memory una = new Distinciones.Pieza[](1);
        una[0] = Distinciones.Pieza({
            nombre: "Pieza 0", obra: 0, juntaArriba: 0, juntaAbajo: 0, esObra: false
        });

        bytes memory initcode =
            abi.encodePacked(type(Distinciones).creationCode, abi.encode(duenio, firmante, una));

        address nuevo;
        uint256 antes = gasleft();
        assembly {
            nuevo := create(0, add(initcode, 32), mload(initcode))
        }
        uint256 gasDeUna = antes - gasleft();

        assertTrue(nuevo != address(0), "el CREATE fallo");
        return (gasDeLas41 - gasDeUna) / 40;
    }

    // --------------------------------------------------------- para mirarlos

    /// @notice Escribe los dos dibujos a `test/salida/`, para poder abrirlos.
    /// @dev No afirma nada que los otros tests no afirmen. Está porque un SVG
    ///      generado se revisa mirándolo, y desarmar dos capas de base64 a mano
    ///      cada vez es la clase de fricción que termina en que nadie lo mire.
    function test_EscribirLosSVGDeMuestra() public {
        vm.writeFile(string.concat(SALIDA, "/pieza.svg"), distinciones.svgDe(_acunar(titular, 12)));
        vm.writeFile(string.concat(SALIDA, "/obra.svg"), distinciones.svgDe(_acunar(titular, 34)));
        vm.writeFile(
            string.concat(SALIDA, "/obra-arcos.svg"), distinciones.svgDe(_acunar(titular, 36))
        );
    }

    // ------------------------------------------------------------------ ayudas

    function _acunar(address a, uint16 pieza) internal returns (uint256) {
        uint64 deadline = _deadline();
        distinciones.mint(a, pieza, deadline, _firmar(PK_FIRMANTE, a, pieza, deadline));
        return distinciones.idDe(a, pieza);
    }

    function _uri(uint256 id) internal view returns (string memory) {
        return distinciones.tokenURI(id);
    }

    /// @dev Desarma el `data:application/json;base64,…`. El base64 no tiene
    ///      comas, así que cortar por la primera y única es suficiente.
    function _json(string memory uri) internal pure returns (string memory) {
        assertEq(vm.indexOf(uri, "data:application/json;base64,"), 0, "el URI no es un data URI");
        return string(Base64.decode(_despuesDeLaComa(uri)));
    }

    function _svg(string memory uri) internal pure returns (string memory) {
        string memory imagen = vm.parseJsonString(_json(uri), ".image");
        assertEq(vm.indexOf(imagen, "data:image/svg+xml;base64,"), 0, "la imagen no es un SVG");
        return string(Base64.decode(_despuesDeLaComa(imagen)));
    }

    function _despuesDeLaComa(string memory s) internal pure returns (string memory) {
        string[] memory partes = vm.split(s, ",");
        assertEq(partes.length, 2, "el data URI tiene mas de una coma");
        return partes[1];
    }

    /// @dev Sólo los contornos, sin el rótulo ni el fondo: lo que hace distinta
    ///      a una pieza de otra aunque las dos se llamaran igual.
    function _paths(string memory svg) internal pure returns (string memory juntos) {
        string[] memory partes = vm.split(svg, '<path d="');
        for (uint256 i = 1; i < partes.length; ++i) {
            juntos = string.concat(juntos, vm.split(partes[i], '"')[0], "|");
        }
    }

    function _cuantosPaths(string memory svg) internal pure returns (uint256) {
        return vm.split(svg, "<path").length - 1;
    }

    function _altoDelViewBox(string memory svg) internal pure returns (uint256) {
        string memory caja = vm.split(vm.split(svg, 'viewBox="0 0 ')[1], '"')[0];
        return vm.parseUint(vm.split(caja, " ")[1]);
    }
}
